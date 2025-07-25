//go:generate ../../../tools/readme_config_includer/generator
package kubernetes

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
)

//go:embed sample.conf
var sampleConfig string

const (
	defaultServiceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// Kubernetes represents the config object for the plugin
type Kubernetes struct {
	URL             string          `toml:"url"`
	BearerToken     string          `toml:"bearer_token"`
	NodeMetricName  string          `toml:"node_metric_name"`
	LabelInclude    []string        `toml:"label_include"`
	LabelExclude    []string        `toml:"label_exclude"`
	ResponseTimeout config.Duration `toml:"response_timeout"`
	Log             telegraf.Logger `toml:"-"`

	tls.ClientConfig

	labelFilter filter.Filter
	httpClient  *http.Client
}

func (*Kubernetes) SampleConfig() string {
	return sampleConfig
}

func (k *Kubernetes) Init() error {
	// If bearer_token is not provided, use the default service account.
	if k.BearerToken == "" {
		k.BearerToken = defaultServiceAccountPath
	}

	labelFilter, err := filter.NewIncludeExcludeFilter(k.LabelInclude, k.LabelExclude)
	if err != nil {
		return err
	}
	k.labelFilter = labelFilter

	if k.URL == "" {
		k.InsecureSkipVerify = true
	}

	if k.NodeMetricName == "" {
		k.NodeMetricName = "kubernetes_node"
	}

	return nil
}

func (k *Kubernetes) Gather(acc telegraf.Accumulator) error {
	if k.URL != "" {
		acc.AddError(k.gatherSummary(k.URL, acc))
		return nil
	}

	var wg sync.WaitGroup
	nodeBaseURLs, err := getNodeURLs(k.Log)
	if err != nil {
		return err
	}

	for _, url := range nodeBaseURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			acc.AddError(k.gatherSummary(url, acc))
		}(url)
	}
	wg.Wait()

	return nil
}

func getNodeURLs(log telegraf.Logger) ([]string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeUrls := make([]string, 0, len(nodes.Items))
	for i := range nodes.Items {
		n := &nodes.Items[i]

		address := getNodeAddress(n.Status.Addresses)
		if address == "" {
			log.Warnf("Unable to node addresses for Node %q", n.Name)
			continue
		}
		nodeUrls = append(nodeUrls, "https://"+address+":10250")
	}

	return nodeUrls, nil
}

// Prefer internal addresses, if none found, use ExternalIP
func getNodeAddress(addresses []v1.NodeAddress) string {
	extAddresses := make([]string, 0)
	for _, addr := range addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
		extAddresses = append(extAddresses, addr.Address)
	}

	if len(extAddresses) > 0 {
		return extAddresses[0]
	}
	return ""
}

func (k *Kubernetes) gatherSummary(baseURL string, acc telegraf.Accumulator) error {
	summaryMetrics := &summaryMetrics{}
	err := k.loadJSON(baseURL+"/stats/summary", summaryMetrics)
	if err != nil {
		return err
	}

	podInfos, err := k.gatherPodInfo(baseURL)
	if err != nil {
		return err
	}
	buildSystemContainerMetrics(summaryMetrics, acc)
	buildNodeMetrics(summaryMetrics, acc, k.NodeMetricName)
	buildPodMetrics(summaryMetrics, podInfos, k.labelFilter, acc)
	return nil
}

func buildSystemContainerMetrics(summaryMetrics *summaryMetrics, acc telegraf.Accumulator) {
	for _, container := range summaryMetrics.Node.SystemContainers {
		tags := map[string]string{
			"node_name":      summaryMetrics.Node.NodeName,
			"container_name": container.Name,
		}
		fields := make(map[string]interface{})
		fields["cpu_usage_nanocores"] = container.CPU.UsageNanoCores
		fields["cpu_usage_core_nanoseconds"] = container.CPU.UsageCoreNanoSeconds
		fields["memory_usage_bytes"] = container.Memory.UsageBytes
		fields["memory_working_set_bytes"] = container.Memory.WorkingSetBytes
		fields["memory_rss_bytes"] = container.Memory.RSSBytes
		fields["memory_page_faults"] = container.Memory.PageFaults
		fields["memory_major_page_faults"] = container.Memory.MajorPageFaults
		fields["rootfs_available_bytes"] = container.RootFS.AvailableBytes
		fields["rootfs_capacity_bytes"] = container.RootFS.CapacityBytes
		fields["logsfs_available_bytes"] = container.LogsFS.AvailableBytes
		fields["logsfs_capacity_bytes"] = container.LogsFS.CapacityBytes
		acc.AddFields("kubernetes_system_container", fields, tags)
	}
}

func buildNodeMetrics(summaryMetrics *summaryMetrics, acc telegraf.Accumulator, metricName string) {
	tags := map[string]string{
		"node_name": summaryMetrics.Node.NodeName,
	}
	fields := make(map[string]interface{})
	fields["cpu_usage_nanocores"] = summaryMetrics.Node.CPU.UsageNanoCores
	fields["cpu_usage_core_nanoseconds"] = summaryMetrics.Node.CPU.UsageCoreNanoSeconds
	fields["memory_available_bytes"] = summaryMetrics.Node.Memory.AvailableBytes
	fields["memory_usage_bytes"] = summaryMetrics.Node.Memory.UsageBytes
	fields["memory_working_set_bytes"] = summaryMetrics.Node.Memory.WorkingSetBytes
	fields["memory_rss_bytes"] = summaryMetrics.Node.Memory.RSSBytes
	fields["memory_page_faults"] = summaryMetrics.Node.Memory.PageFaults
	fields["memory_major_page_faults"] = summaryMetrics.Node.Memory.MajorPageFaults
	fields["network_rx_bytes"] = summaryMetrics.Node.Network.RXBytes
	fields["network_rx_errors"] = summaryMetrics.Node.Network.RXErrors
	fields["network_tx_bytes"] = summaryMetrics.Node.Network.TXBytes
	fields["network_tx_errors"] = summaryMetrics.Node.Network.TXErrors
	fields["fs_available_bytes"] = summaryMetrics.Node.FileSystem.AvailableBytes
	fields["fs_capacity_bytes"] = summaryMetrics.Node.FileSystem.CapacityBytes
	fields["fs_used_bytes"] = summaryMetrics.Node.FileSystem.UsedBytes
	fields["runtime_image_fs_available_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.AvailableBytes
	fields["runtime_image_fs_capacity_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.CapacityBytes
	fields["runtime_image_fs_used_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.UsedBytes
	acc.AddFields(metricName, fields, tags)
}

func (k *Kubernetes) gatherPodInfo(baseURL string) ([]item, error) {
	var podAPI pods
	err := k.loadJSON(baseURL+"/pods", &podAPI)
	if err != nil {
		return nil, err
	}
	podInfos := make([]item, 0, len(podAPI.Items))
	podInfos = append(podInfos, podAPI.Items...)
	return podInfos, nil
}

func (k *Kubernetes) loadJSON(url string, v interface{}) error {
	var req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	var resp *http.Response
	tlsCfg, err := k.ClientConfig.TLSConfig()
	if err != nil {
		return err
	}

	if k.httpClient == nil {
		if k.ResponseTimeout < config.Duration(time.Second) {
			k.ResponseTimeout = config.Duration(time.Second * 5)
		}
		k.httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsCfg,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: time.Duration(k.ResponseTimeout),
		}
	}

	// Read bearer token from file and use it for authorization
	var bearerTokenString string
	if k.BearerToken != "" {
		token, err := os.ReadFile(k.BearerToken)
		if err != nil {
			return err
		}
		bearerTokenString = strings.TrimSpace(string(token))
	}
	req.Header.Set("Authorization", "Bearer "+bearerTokenString)
	req.Header.Add("Accept", "application/json")
	resp, err = k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making HTTP request to %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP status %s", url, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	return nil
}

func buildPodMetrics(summaryMetrics *summaryMetrics, podInfo []item, labelFilter filter.Filter, acc telegraf.Accumulator) {
	for _, pod := range summaryMetrics.Pods {
		podLabels := make(map[string]string)
		containerImages := make(map[string]string)
		for _, info := range podInfo {
			if info.Metadata.Name == pod.PodRef.Name && info.Metadata.Namespace == pod.PodRef.Namespace {
				for _, v := range info.Spec.Containers {
					containerImages[v.Name] = v.Image
				}
				for k, v := range info.Metadata.Labels {
					if labelFilter.Match(k) {
						podLabels[k] = v
					}
				}
			}
		}

		for _, container := range pod.Containers {
			tags := map[string]string{
				"node_name":      summaryMetrics.Node.NodeName,
				"namespace":      pod.PodRef.Namespace,
				"container_name": container.Name,
				"pod_name":       pod.PodRef.Name,
			}
			for k, v := range containerImages {
				if k == container.Name {
					tags["image"] = v
					tok := strings.Split(v, ":")
					if len(tok) == 2 {
						tags["version"] = tok[1]
					}
				}
			}
			for k, v := range podLabels {
				tags[k] = v
			}
			fields := make(map[string]interface{})
			fields["cpu_usage_nanocores"] = container.CPU.UsageNanoCores
			fields["cpu_usage_core_nanoseconds"] = container.CPU.UsageCoreNanoSeconds
			fields["memory_usage_bytes"] = container.Memory.UsageBytes
			fields["memory_working_set_bytes"] = container.Memory.WorkingSetBytes
			fields["memory_rss_bytes"] = container.Memory.RSSBytes
			fields["memory_page_faults"] = container.Memory.PageFaults
			fields["memory_major_page_faults"] = container.Memory.MajorPageFaults
			fields["rootfs_available_bytes"] = container.RootFS.AvailableBytes
			fields["rootfs_capacity_bytes"] = container.RootFS.CapacityBytes
			fields["rootfs_used_bytes"] = container.RootFS.UsedBytes
			fields["logsfs_available_bytes"] = container.LogsFS.AvailableBytes
			fields["logsfs_capacity_bytes"] = container.LogsFS.CapacityBytes
			fields["logsfs_used_bytes"] = container.LogsFS.UsedBytes
			acc.AddFields("kubernetes_pod_container", fields, tags)
		}

		for _, volume := range pod.Volumes {
			tags := map[string]string{
				"node_name":   summaryMetrics.Node.NodeName,
				"pod_name":    pod.PodRef.Name,
				"namespace":   pod.PodRef.Namespace,
				"volume_name": volume.Name,
			}
			for k, v := range podLabels {
				tags[k] = v
			}
			fields := make(map[string]interface{})
			fields["available_bytes"] = volume.AvailableBytes
			fields["capacity_bytes"] = volume.CapacityBytes
			fields["used_bytes"] = volume.UsedBytes
			acc.AddFields("kubernetes_pod_volume", fields, tags)
		}

		tags := map[string]string{
			"node_name": summaryMetrics.Node.NodeName,
			"pod_name":  pod.PodRef.Name,
			"namespace": pod.PodRef.Namespace,
		}
		for k, v := range podLabels {
			tags[k] = v
		}
		fields := make(map[string]interface{})
		fields["rx_bytes"] = pod.Network.RXBytes
		fields["rx_errors"] = pod.Network.RXErrors
		fields["tx_bytes"] = pod.Network.TXBytes
		fields["tx_errors"] = pod.Network.TXErrors
		acc.AddFields("kubernetes_pod_network", fields, tags)
	}
}

func init() {
	inputs.Add("kubernetes", func() telegraf.Input {
		return &Kubernetes{
			LabelExclude: []string{"*"},
		}
	})
}
