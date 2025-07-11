package round

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/testutil"
)

// Verifies that values are rounded correctly
func TestRound_PositivePrecision(t *testing.T) {
	tests := []struct {
		name     string
		input    []telegraf.Metric
		expected []telegraf.Metric
	}{
		{
			name: "float64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(5567.56356)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(-1043.245956459)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(5567.56)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(-1043.25)},
					time.Unix(0, 0),
				),
			},
		},
		{
			name: "uint64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(2505)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(2505)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
			},
		},
		{
			name: "int64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(16594)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(-34437)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(16594)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(-34437)},
					time.Unix(0, 0),
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := Round{
				Precision: 2,
				Log:       testutil.Logger{},
			}
			require.NoError(t, plugin.Init())

			actual := plugin.Apply(tt.input...)
			testutil.RequireMetricsEqual(t, tt.expected, actual)
		})
	}
}

// Verifies that values are rounded correctly
func TestRound_NegativePrecision(t *testing.T) {
	tests := []struct {
		name     string
		input    []telegraf.Metric
		expected []telegraf.Metric
	}{
		{
			name: "float64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(5567.56356)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(-1043.245956459)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(5600)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": float64(-1000)},
					time.Unix(0, 0),
				),
			},
		},
		{
			name: "uint64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(2505)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(2500)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
			},
		},
		{
			name: "int64",
			input: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(16594)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(-34437)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(16600)},
					time.Unix(0, 0),
				),
				metric.New("cpu",
					map[string]string{},
					map[string]interface{}{"value": int64(-34400)},
					time.Unix(0, 0),
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := Round{
				Precision: -2,
				Log:       testutil.Logger{},
			}
			require.NoError(t, plugin.Init())

			actual := plugin.Apply(tt.input...)
			testutil.RequireMetricsEqual(t, tt.expected, actual)
		})
	}
}

// Verifies that round() returns zero values as 0
func TestRoundWithZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		input    []telegraf.Metric
		expected []telegraf.Metric
	}{
		{
			name: "zeros",
			input: []telegraf.Metric{
				metric.New("zero_uint64",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
				metric.New("zero_int64",
					map[string]string{},
					map[string]interface{}{"value": int64(0)},
					time.Unix(0, 0),
				),
				metric.New("zero_float",
					map[string]string{},
					map[string]interface{}{"value": float64(0.0)},
					time.Unix(0, 0),
				),
			},
			expected: []telegraf.Metric{
				metric.New("zero_uint64",
					map[string]string{},
					map[string]interface{}{"value": uint64(0)},
					time.Unix(0, 0),
				),
				metric.New("zero_int64",
					map[string]string{},
					map[string]interface{}{"value": int64(0)},
					time.Unix(0, 0),
				),
				metric.New("zero_float",
					map[string]string{},
					map[string]interface{}{"value": float64(0.0)},
					time.Unix(0, 0),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := Round{
				Precision: 1,
				Log:       testutil.Logger{},
			}
			require.NoError(t, plugin.Init())

			actual := plugin.Apply(tt.input...)
			testutil.RequireMetricsEqual(t, tt.expected, actual)
		})
	}
}

func TestTracking(t *testing.T) {
	// Setup raw input and expected output
	inputRaw := []telegraf.Metric{
		metric.New(
			"uint64",
			map[string]string{},
			map[string]interface{}{"value": uint64(1236)},
			time.Unix(0, 0),
		),
		metric.New(
			"int64",
			map[string]string{},
			map[string]interface{}{"value": int64(-234)},
			time.Unix(0, 0),
		),
		metric.New(
			"float",
			map[string]string{},
			map[string]interface{}{"value": float64(-45.7894)},
			time.Unix(0, 0),
		),
	}

	expected := []telegraf.Metric{
		metric.New(
			"uint64",
			map[string]string{},
			map[string]interface{}{"value": uint64(1236)},
			time.Unix(0, 0),
		),
		metric.New(
			"int64",
			map[string]string{},
			map[string]interface{}{"value": int64(-234)},
			time.Unix(0, 0),
		),
		metric.New(
			"float",
			map[string]string{},
			map[string]interface{}{"value": float64(-45.79)},
			time.Unix(0, 0),
		),
	}

	// Create fake notification for testing
	var mu sync.Mutex
	delivered := make([]telegraf.DeliveryInfo, 0, len(inputRaw))
	notify := func(di telegraf.DeliveryInfo) {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, di)
	}

	// Convert raw input to tracking metric
	input := make([]telegraf.Metric, 0, len(inputRaw))
	for _, m := range inputRaw {
		tm, _ := metric.WithTracking(m, notify)
		input = append(input, tm)
	}

	// Prepare and start the plugin
	plugin := &Round{
		Precision: 2,
		Log:       testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	// Process expected metrics and compare with resulting metrics
	actual := plugin.Apply(input...)
	testutil.RequireMetricsEqual(t, expected, actual)

	// Simulate output acknowledging delivery
	for _, m := range actual {
		m.Accept()
	}

	// Check delivery
	require.Eventuallyf(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(input) == len(delivered)
	}, time.Second, 100*time.Millisecond, "%d delivered but %d expected", len(delivered), len(expected))
}
