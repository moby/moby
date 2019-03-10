package stats // import "github.com/docker/docker/daemon/stats"

import (
	"reflect"
	"testing"
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

	for index := range testFifos {
		result := fifoUint(testFifos[index], testValues[index], testSizes[index])
		if !reflect.DeepEqual(result, expected[index]) {
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

	for index := range testFifos {
		result := fifoFloat(testFifos[index], testValues[index], testSizes[index])
		if !reflect.DeepEqual(result, expected[index]) {
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

	for index, test := range testArrays {
		result := lowestOf(test)
		if result != expected[index] {
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

	for index, test := range testArrays {
		result := highestOf(test)
		if result != expected[index] {
			t.Fail()
		}
	}
}

func TestPercent(t *testing.T) {
	testPercents := []int{0, 100, 123141412, -1, -100}

	expected := []int{0, 1, 1231414, 0, -1}

	for index, test := range testPercents {
		result := percent(test)
		if result != expected[index] {
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

	for index, test := range testPercentages {
		result := percentageBetween(test.old, test.new)
		if result != expected[index] {
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

	for index, test := range testCPUUsage {
		config, number := CPUUsageToConfig(test)
		if config != expected[index].config || number != expected[index].number {
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

	for index := range testMemoryWeight {
		result := generateMemoryWeight(testMemoryWeight[index], testHighest[index])
		if !reflect.DeepEqual(result, expected[index]) {
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

	for index := range testAverrages {
		result := weightedAverrage(testAverrages[index], testWeights[index])
		if !reflect.DeepEqual(result, expected[index]) {
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

	for index, test := range testAverrages {
		result := averrageFloat(test)
		if float32(result) != float32(expected[index]) {
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

	for index, test := range testAverrages {
		result := averrage(test)
		if result != expected[index] {
			t.Fail()
		}
	}
}
