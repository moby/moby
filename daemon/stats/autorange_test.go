package stats // import "github.com/docker/docker/daemon/stats"

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

func TestFifoUint(t *testing.T) {
	testFifos := [][]uint64{
		{0, 1, 2, 3, 4},
		{23, 235, 554, 34, 123456},
		{},
		{0, 1, 2, 3, 4},
	}

	testSizes := []int{5, 5, 0, 6}

	testValues := []uint64{12, 12, 12, 12}

	expected := [][]uint64{
		{1, 2, 3, 4, 12},
		{235, 554, 34, 123456, 12},
		{},
		{0, 1, 2, 3, 4, 12},
	}

	for idx := range testFifos {
		result := fifoUint(testFifos[idx], testValues[idx], testSizes[idx])
		if !reflect.DeepEqual(result, expected[idx]) {
			t.Fail()
		}
	}
}

func TestFifoFloat(t *testing.T) {
	testFifos := [][]float64{
		{1.2, 4.2, .42, 104.2},
		{},
		{1.2, 4.2, .42, 104.2},
	}

	testSizes := []int{4, 0, 10}

	testValues := []float64{12.42, 12.42, 12.42}

	expected := [][]float64{
		{4.2, .42, 104.2, 12.42},
		{},
		{1.2, 4.2, .42, 104.2, 12.42},
	}

	for idx := range testFifos {
		result := fifoFloat(testFifos[idx], testValues[idx], testSizes[idx])
		if !reflect.DeepEqual(result, expected[idx]) {
			t.Fail()
		}
	}
}

func TestLowestOf(t *testing.T) {
	testArrays := [][]uint64{
		{1, 42, 131, 2313213},
		{0, 0, 0, 0},
		{0, 1, 9, 12},
	}

	expected := []uint64{1, 0, 0}

	for idx, test := range testArrays {
		result := lowestOf(test)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestHighestOf(t *testing.T) {
	testArrays := [][]uint64{
		{1, 42, 131, 2313213},
		{0, 0, 0, 0},
		{0, 1, 9, 12},
		{1},
	}

	expected := []int{2313213, 0, 12, 1}

	for idx, test := range testArrays {
		result := highestOf(test)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestPercent(t *testing.T) {
	testPercents := []int{0, 100, 123141412, -1, -100}

	expected := []int{0, 1, 1231414, 0, -1}

	for idx, test := range testPercents {
		result := percent(test)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestPercentageBetween(t *testing.T) {
	type testPercentage struct {
		old, new int
	}

	testPercentages := []testPercentage{
		{
			old: 12,
			new: 42,
		},
		{
			old: 42,
			new: 12,
		},
		{
			old: -1,
			new: 1,
		},
		{
			old: 0,
			new: 0,
		},
		{
			old: 1,
			new: 2,
		},
	}

	expected := []int{250, -71, -200, 0, 100}

	for idx, test := range testPercentages {
		result := percentageBetween(test.old, test.new)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestIsActivated(t *testing.T) {
	ar := AutoRangeWatcher{}
	ar.Config = swarm.AutoRange{
		"memory": make(map[string]string),
		"cpu%":   make(map[string]string),
	}
	testCases := []string{"memory", "cpu%", "blabla"}

	expected := []bool{true, true, false}

	for idx, test := range testCases {
		result := ar.IsActivated(test)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestContinueIteration(t *testing.T) {
	testCases := []struct {
		category, toTest string
		done             bool
	}{
		{
			category: "cpu%",
			toTest:   "cpu%",
			done:     false,
		},
		{
			category: "memory",
			toTest:   "MEMORY",
			done:     false,
		},
		{
			category: "blabla",
			toTest:   "blabla",
			done:     true,
		},
	}

	expected := []bool{true, false, false}

	for idx, test := range testCases {
		result := continueIteration(test.category, test.toTest, test.done)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestCPUUsageToConfig(t *testing.T) {
	testCPUUsage := []string{"123", "1212", "0", "-1"}

	type Expected struct {
		config, number string
	}
	expected := []Expected{
		{
			config: "0,1",
			number: "2",
		},
		{
			config: "0,1,2,3,4,5,6,7,8,9,10,11,12",
			number: "13",
		},
		{
			config: "",
			number: "",
		},
		{
			config: "",
			number: "",
		},
	}

	for idx, test := range testCPUUsage {
		config, number := CPUUsageToConfig(test)
		if config != expected[idx].config || number != expected[idx].number {
			t.Fail()
		}
	}
}

func TestIsStarted(t *testing.T) {
	ars := []AutoRangeWatcher{
		{
			started: true,
		},
		{
			started: false,
		},
	}

	expected := []bool{true, false}

	for idx, ar := range ars {
		result := ar.isStarted()
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestGetExtremeValues(t *testing.T) {
	testValues := []struct {
		usage, lowest, highest uint64
	}{
		{
			usage:   1234,
			lowest:  1288,
			highest: 1352,
		},
		{
			usage:   1853,
			lowest:  1200,
			highest: 1534,
		},
		{
			usage:   4242,
			lowest:  4242,
			highest: 12345,
		},
	}

	expected := []struct {
		lowest, highest uint64
	}{
		{
			lowest:  1234,
			highest: 1352,
		},
		{
			lowest:  1200,
			highest: 1853,
		},
		{
			lowest:  4242,
			highest: 12345,
		},
	}

	for idx, test := range testValues {
		lowest, highest := getExtremeValues(test.usage, test.lowest, test.highest)
		if lowest != expected[idx].lowest || highest != expected[idx].highest {
			t.Fail()
		}
	}
}

func TestCheckMemoryEndCondition(t *testing.T) {
	testCases := []struct {
		lenSerie, limit int
		mediumAmplitude uint64
	}{
		{
			lenSerie:        42,
			limit:           40,
			mediumAmplitude: 64,
		},
		{
			lenSerie:        42,
			limit:           40,
			mediumAmplitude: 2,
		},
		{
			lenSerie:        10,
			limit:           20,
			mediumAmplitude: 2,
		},
		{
			lenSerie:        10,
			limit:           19,
			mediumAmplitude: 2,
		},
	}

	expected := []bool{true, true, false, true}

	for idx, test := range testCases {
		result := checkMemoryEndCondition(test.lenSerie, test.limit, test.mediumAmplitude)
		if result != expected[idx] {
			t.Fail()
		}
	}
}

func TestBaseValueMemory(t *testing.T) {
	ars := []AutoRangeWatcher{
		{
			Config: map[string]map[string]string{
				"memory": {
					"min":       "0",
					"max":       "0",
					"threshold": "0",
				},
			},
		},
		{
			Config: map[string]map[string]string{
				"memory": {
					"min":       "12340",
					"max":       "23450",
					"threshold": "20",
				},
			},
		},
		{
			Config: map[string]map[string]string{
				"cpu": {
					"min":       "0",
					"max":       "0",
					"threshold": "0",
				},
			},
		},
	}

	expected := []struct {
		min, max, threshold int
	}{
		{
			min:       10000,
			max:       20000,
			threshold: 10,
		},
		{
			min:       12340,
			max:       23450,
			threshold: 20,
		},
		{
			min:       0,
			max:       0,
			threshold: 0,
		},
	}

	for idx, ar := range ars {
		min, max, threshold := ar.baseValueMemory()
		if min != expected[idx].min || max != expected[idx].max || threshold != expected[idx].threshold {
			t.Fail()
		}
	}
}

func TestBaseValueCPU(t *testing.T) {
	ars := []AutoRangeWatcher{
		{
			Config: map[string]map[string]string{
				"cpu%": {
					"min": "0",
					"max": "0",
				},
			},
		},
		{
			Config: map[string]map[string]string{
				"cpu%": {
					"min": "60",
					"max": "90",
				},
			},
		},
		{
			Config: map[string]map[string]string{
				"memory": {
					"min": "10",
					"max": "20",
				},
			},
		},
	}

	expected := []struct {
		min, max int
	}{
		{
			min: 0,
			max: 0,
		},
		{
			min: 60,
			max: 90,
		},
		{
			min: 0,
			max: 0,
		},
	}

	for idx, ar := range ars {
		min, max := ar.baseValueCPU()
		if min != expected[idx].min || max != expected[idx].max {
			t.Fail()
		}
	}
}

func TestGenerateMemoryWeight(t *testing.T) {
	testMemoryWeight := [][]uint64{
		{64, 24, 12, 121},
		{1264, 13234, 934958, 7287371},
		{},
		{0, 0, 0, 0},
		{1, 1, 1, 1},
	}

	testHighest := [][]uint64{
		{132},
		{8287371},
		{123},
		{1},
		{1},
	}

	expected := [][]float32{
		{0.5, 0.2, 0.09090909, 1},
		{0.00015253203, 0.0015974441, 0.125, 1},
		{},
		{},
		{1, 1, 1, 1},
	}

	for idx := range testMemoryWeight {
		result := generateMemoryWeight(testMemoryWeight[idx], testHighest[idx])
		if !reflect.DeepEqual(result, expected[idx]) {
			t.Fail()
		}
	}
}

func TestWeightedAverrage(t *testing.T) {
	testAverrages := [][]uint64{
		{1264, 13234, 934958, 7287371},
		{64, 24, 12, 121},
		{},
		{0, 0, 0, 0},
		{1, 1, 1, 1},
	}

	testWeights := [][]float32{
		{0.00015253203, 0.0015974441, 0.125, 1},
		{0.5, 0.2, 0.09090909, 1},
		{},
		{0, 0, 0, 0},
		{1, 1, 1, 1},
	}

	expected := []int{7834575, 125, 0, 0, 1}

	for idx := range testAverrages {
		result := weightedAverrage(testAverrages[idx], testWeights[idx])
		if !reflect.DeepEqual(result, expected[idx]) {
			t.Fail()
		}
	}
}

func TestAverrageFloat(t *testing.T) {
	testAverrages := [][]float64{
		{1.42, 123.09, 32.0, 1.2},
		{1, 2, 3, 4, 5},
		{},
		{0, 0, 0, 0, 0},
	}

	expected := []float64{39.427500, 3.000000, 0.000000, 0.000000}

	for idx, test := range testAverrages {
		result := averrageFloat(test)
		if float32(result) != float32(expected[idx]) {
			t.Fail()
		}
	}
}

func TestAverrage(t *testing.T) {
	testAverrages := [][]uint64{
		{1, 123, 32, 2},
		{0, 0, 1, 1},
		{},
		{0, 0, 0, 0},
	}

	expected := []uint64{39, 0, 0, 0}

	for idx, test := range testAverrages {
		result := averrage(test)
		if result != expected[idx] {
			t.Fail()
		}
	}
}
