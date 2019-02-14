package stats // import "github.com/docker/docker/daemon/stats"

// #AutoRange
// a feature that help the user predict and apply the best limits to his services.
// 	Why?
// This collector extension was thought as a way to monitor and predict the optimal configuration
// for a service.
// The goal was to find the point where a service could function properly, but still save as much
// resources as possible, by monitoring activity and deducing optimal values.
// It was written as a way to answer the question
//	 `How to optimise the number of services running on our infrastructure without losing quality of service?`

// How?
// It uses swarm labels and require swarm mode to be enabled (see #improvements).
// The logic behind the feature can be described in 3 points:
// - First, we collect the metrics and apply transformations on it to generate two values.
// Those values represent a “box” around the actual consumption.
// - Then, we transform these values into timeseries, using some of the keydata collected previously to weight our operations.
// The amplitude of change between values is monitored to know if it’s a good time to stop measurements.
// - Finally, we obtain refined values that we apply as limitation to the service.
// The data is then kept in a reduced form to limit memory usage.
// The functionnality is declared by adding the autorange key to the docker-compose.yml.
// The mechanism works for cpu% and memory, with or without basevalues.
// Below is an example of both.
// autorange:
// memory:
// cpu%:
// The available keys are:- min (in octets)- max (in octets)- threshold% (only for memory, represents a security margin that will be refined by the algorithm)
// autorange:
// memory:
//         min: "110000"
// 		   max: "120000"
// 		   threshold%: "10"
// cpu%:
// 		   min: "60"
// 		   max: "70"
// This functionality is deployed with docker stack deploy --compose-file=/your/compose/file and
// then docker container stats --format AutoRange(format is not necessary but shows the predicted values).
//          The `docker container stats` command is mandatory to start and keep running the collector.
// You can always leave the docker container stats screen and
// come back later, the mechanism will be paused and the accumulated datas won’t be lost.

import (
	"math"
	"strconv"
	"strings"
	"time"

	"context"

	ctn "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
)

const (
	kiB                   = 1024
	miB                   = 1024 * kiB
	giB                   = 1024 * miB
	tiB                   = 1024 * giB
	minAllowedMemoryLimit = 5 * miB
)

// PredictedValueRAM is a struct that holds the values used to generate the memory limits
type PredictedValueRAM struct {
	min, max, threshold []uint64
}

// TimeSerieRAM is a struct that holds everything needed for memory limit generation
type TimeSerieRAM struct {
	min, max, usage, highest, lowest, amplitude []uint64
	timestamps                                  []time.Time
	started                                     time.Time
	PredictedValues                             PredictedValueRAM
	MemoryPrediction                            bool
}

// PredictedValueCPU is a struct that holds the values used to generate the CPU limits
type PredictedValueCPU struct {
	percent, usage []float64
}

// TimeSerieCPU is a struct that holds everything needed for CPU limit generation
type TimeSerieCPU struct {
	percent, usage  []float64
	timestamps      []time.Time
	started         time.Time
	PredictedValues PredictedValueCPU
	CPUPrediction   bool
}

// Observor is the struct that holds the timeserie for the watcher
type Observor struct {
	TimeSerieRAM
	TimeSerieCPU
}

// AutoRangeWatcher is the struct that holds everything required to enable autorange functionnality
type AutoRangeWatcher struct {
	Output, Input chan types.StatsJSON
	WaitChan      chan bool
	TickRate      time.Duration
	Config        swarm.AutoRange
	Target        *container.Container
	ServiceName   string
	Obs           *Observor
	Ctx           context.Context
	Limit         int
	Finished      bool
}

// NewObservor returns a newly initialized observor that will be used by the watcher
func NewObservor(size int) *Observor {
	return &Observor{
		TimeSerieRAM: TimeSerieRAM{
			min:              make([]uint64, 0, size),
			max:              make([]uint64, 0, size),
			usage:            make([]uint64, 0, size),
			highest:          make([]uint64, 0, size),
			lowest:           make([]uint64, 0, size),
			amplitude:        make([]uint64, 0, size),
			MemoryPrediction: false,
			PredictedValues: PredictedValueRAM{
				min:       make([]uint64, 0, size),
				max:       make([]uint64, 0, size),
				threshold: make([]uint64, 0, size),
			},
		},
		TimeSerieCPU: TimeSerieCPU{
			percent:       make([]float64, 0, size),
			usage:         make([]float64, 0, size),
			CPUPrediction: false,
			PredictedValues: PredictedValueCPU{
				percent: make([]float64, 0, size),
				usage:   make([]float64, 0, size),
			},
		},
	}
}

func fifoUint(arr []uint64, newv uint64, limit int) []uint64 {
	if len(arr) < limit {
		arr = append(arr, newv)
		return arr
	}
	_, arr = arr[0], arr[1:]
	arr = append(arr, newv)
	return arr
}

func fifoFloat(arr []float64, newv float64, limit int) []float64 {
	if len(arr) < limit {
		arr = append(arr, newv)
		return arr
	}
	_, arr = arr[0], arr[1:]
	arr = append(arr, newv)
	return arr
}

func lowestOf(arr []uint64) uint64 {
	lowest := arr[0]

	for _, val := range arr {
		if val < lowest {
			lowest = val
		}
	}
	return lowest
}

func highestOf(arr []uint64) int {
	highest := arr[0]

	for _, val := range arr {
		if val > highest {
			highest = val
		}
	}
	return int(highest)
}

func percent(value int) int {
	return value / 100
}

func percentageBetween(old, new int) (delta int) {
	diff := float64(new - old)
	delta = int((diff / float64(old)) * 100)
	return
}

func percentOf(f, s int) int {
	return s / f
}

// SetNewContext helps for the start and stops of the collector
func (ar *AutoRangeWatcher) SetNewContext(ctx context.Context) {
	ar.Ctx = ctx
	ar.WaitChan <- true
}

// UpdateResources is the function that handle the verification and application of the generated limits
func (ar *AutoRangeWatcher) UpdateResources() error {
	var update ctn.UpdateConfig

	if _, exist := ar.Config["memoryAR"]; exist {

		sugMin, _ := strconv.Atoi(ar.Config["memoryAR"]["nmin"])
		sugMax, _ := strconv.Atoi(ar.Config["memoryAR"]["nmax"])
		threshold, _ := strconv.Atoi(ar.Config["memoryAR"]["opti"])

		// One last sum with the highest usage to smooth the prediction and reduce the
		// error probability. It's generaly a subtle ajustement.
		// The docker daemon does not permit memory limit lesser than 6mb

		update.Resources.Memory = int64((sugMax + highestOf(ar.Obs.TimeSerieRAM.highest)) / 2)
		if update.Resources.Memory < minAllowedMemoryLimit {
			update.Resources.Memory = minAllowedMemoryLimit + miB
		}

		// Memoryswap should always be greater than memory limit, but can be illimited (-1)
		if int64(sugMax*threshold) <= update.Resources.Memory {
			update.Resources.MemorySwap = update.Resources.Memory + 1
		} else {
			update.Resources.MemorySwap = int64(sugMax + (percent(sugMax) * threshold))
		}

		// Here we do pretty much the same as above, to further refine the limit and better fit
		// the observed consumption
		update.Resources.MemoryReservation = int64((uint64(sugMin) + lowestOf(ar.Obs.TimeSerieRAM.lowest)))
		if update.Resources.MemoryReservation/2 < minAllowedMemoryLimit {
			update.Resources.MemoryReservation = minAllowedMemoryLimit + 5*miB
		}

		if update.Resources.MemoryReservation > update.Resources.Memory {
			update.Resources.MemoryReservation, update.Resources.Memory = update.Resources.Memory, update.Resources.MemoryReservation
		}

		if update.Resources.MemorySwap < update.Resources.Memory {
			update.Resources.MemorySwap = -1
		}

		ar.Config["memoryAR"]["sugmin"] = strconv.Itoa(int(update.Resources.MemoryReservation))
		ar.Config["memoryAR"]["sugmax"] = strconv.Itoa(int(update.Resources.Memory))

	}

	if _, exist := ar.Config["cpuAR"]; exist {

		sugMax, _ := strconv.Atoi(ar.Config["cpuAR"]["usageOpti"])

		update.Resources.CPURealtimeRuntime = int64(sugMax)
		update.Resources.CpusetCpus, ar.Config["cpuAR"]["numCPU"] = nSetCPU(ar.Config["cpuAR"]["percentOpti"])

	}

	ar.Obs = nil

	// Updating is done using the docker client API
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	_, err = cli.ContainerUpdate(ar.Ctx, ar.Target.ID, update)
	if err != nil {
		return err
	}
	logrus.Infof("container: %s (service: %s) now has limits applicated\n", ar.Target.Name, ar.ServiceName)

	return nil
}

func nSetCPU(cpus string) (string, string) {
	var cpuConfig string

	pcpus, _ := strconv.ParseFloat(cpus, 32)
	n := int(pcpus/100) + 1
	for i := 0; i < n; i++ {
		cpuConfig += strconv.Itoa(i)
		if i+1 < n {
			cpuConfig += ","
		}
	}

	return cpuConfig, strconv.Itoa(n)
}

// Watch is the function that will keep the goroutine alive, process the metrics and generate the time series
func (ar *AutoRangeWatcher) Watch() {

	var (
		input                                    types.StatsJSON
		lowest, highest, oldUsage, oldSystem     uint64 = 0, 0, 0, 0
		min, max, threshold, cpuTurn, memoryTurn int    = 0, 0, 0, 0, 0
		cpuMin, cpuMax                           int
	)

	// Recover base config, those values will be used as base values
	if _, exist := ar.Config["memory"]; exist {
		min, _ = strconv.Atoi(ar.Config["memory"]["min"])
		if min == 0 {
			min = 1
		}

		max, _ = strconv.Atoi(ar.Config["memory"]["max"])
		if max == 0 {
			max = 1
		}

		threshold, _ = strconv.Atoi(ar.Config["memory"]["threshold"])
		if threshold == 0 {
			threshold = 10
		}
		ar.Config["memoryAR"] = make(map[string]string)
	}

	if _, exist := ar.Config["cpu%"]; exist {
		cpuMin, _ = strconv.Atoi(ar.Config["cpu%"]["min"])
		cpuMax, _ = strconv.Atoi(ar.Config["cpu%"]["max"])
		if cpuMin != 0 && cpuMax != 0 {
			fifoFloat(ar.Obs.TimeSerieCPU.percent, float64((cpuMin+cpuMax)/2), ar.Limit)
		}
		ar.Config["cpuAR"] = make(map[string]string)
	}

	// Initialisation time
	ticker := time.NewTicker(ar.TickRate)
	time.Sleep(ar.TickRate)
	started := false

	logrus.Infof("container: %s (service: %s) started with activated autorange\n", ar.Target.Name, ar.ServiceName)
	for range ticker.C {
		select {
		case in := <-ar.Input:
			input = in
		case <-ar.Ctx.Done(): // Handler for signal interrupt
			<-ar.WaitChan
			continue
		}

		// Healthchecking is required before every loops to ensure data integrity
		// We don't want false prediction because the container was offline
		if ar.Target.State.Dead || !ar.Target.State.Running {
			logrus.Infof("container: %s (service: %s) exited, removing autorange\n", ar.Target.Name, ar.ServiceName)
			return
		}

		// Initalisation / End routines
		if !started {
			input.Stats.MemoryStats.MaxUsage, lowest = input.Stats.MemoryStats.Usage, input.Stats.MemoryStats.Usage
			started = true
		} else if ar.Obs.TimeSerieRAM.MemoryPrediction && ar.Obs.TimeSerieCPU.CPUPrediction && !ar.Finished {
			ar.Finished = true

			err := ar.UpdateResources()
			if err != nil {
				logrus.Errorf("err: %v\n", err)
			}
			return
		}

		for category := range ar.Config {
			if strings.Compare(category, "memory") == 0 && !ar.Obs.TimeSerieRAM.MemoryPrediction {

				// Follow memory usage and change min and max accordingly.
				// These values represent the "bearings" around the usage value
				min, max = processMemoryStats(input.Stats.MemoryStats, min, max, threshold)

				// Always get the lowest and highest point in the serie,
				// as we'll use them for weighting purposes
				if input.Stats.MemoryStats.Usage < lowest {
					lowest = input.Stats.MemoryStats.Usage
				} else if input.Stats.MemoryStats.Usage > highest {
					highest = input.Stats.MemoryStats.Usage
				}

				ar.Obs.TimeSerieRAM.min = fifoUint(ar.Obs.TimeSerieRAM.min, uint64(min), ar.Limit)
				ar.Obs.TimeSerieRAM.max = fifoUint(ar.Obs.TimeSerieRAM.max, uint64(max), ar.Limit)
				ar.Obs.TimeSerieRAM.usage = fifoUint(ar.Obs.TimeSerieRAM.usage, input.Stats.MemoryStats.Usage, ar.Limit)

				// Timeserie arrays are ready to be processed
				if memoryTurn >= ar.Limit {
					memoryTurn = 0

					// Stats about the serie
					// Amplitude represent the space between lowest and highest
					ar.Obs.TimeSerieRAM.highest = fifoUint(ar.Obs.TimeSerieRAM.highest, highest, ar.Limit)
					ar.Obs.TimeSerieRAM.lowest = fifoUint(ar.Obs.TimeSerieRAM.lowest, lowest, ar.Limit)
					ar.Obs.TimeSerieRAM.amplitude = fifoUint(ar.Obs.TimeSerieRAM.amplitude, uint64(percentOf(int(lowest), int(highest))), ar.Limit)

					// Generate predicted values
					aMin, aMax := averrage(ar.Obs.TimeSerieRAM.min), averrage(ar.Obs.TimeSerieRAM.max)
					aMin = aMin + (aMin/100)*uint64(percentageBetween(int(aMin), int(lowest)))
					aMax = aMax + (aMax/100)*uint64(percentageBetween(int(aMax), int(highest)))

					// Stock predicted values
					ar.Obs.TimeSerieRAM.PredictedValues.min = fifoUint(ar.Obs.TimeSerieRAM.PredictedValues.min, aMin, ar.Limit)
					ar.Obs.TimeSerieRAM.PredictedValues.max = fifoUint(ar.Obs.TimeSerieRAM.PredictedValues.max, aMax, ar.Limit)

					highest, lowest = 0, input.Stats.MemoryStats.Usage

					// When the number of timeseries is big enough, or if the rate of change <= 2
					// we can assume that the optimal limits can be calculated
					medAmplitude, lenSerie := averrage(ar.Obs.TimeSerieRAM.amplitude), len(ar.Obs.TimeSerieRAM.PredictedValues.min)
					ar.Obs.TimeSerieRAM.PredictedValues.threshold = fifoUint(ar.Obs.TimeSerieRAM.PredictedValues.threshold, medAmplitude, ar.Limit)
					if lenSerie >= ar.Limit || (lenSerie > ar.Limit/2 && medAmplitude <= 2) {

						// This flag is set to stop data gathering and enable limit application
						ar.Obs.TimeSerieRAM.MemoryPrediction = true
					}

					// Display result
					ar.Config["memoryAR"]["nmin"] = strconv.Itoa(wAverrage(ar.Obs.TimeSerieRAM.PredictedValues.min, generateMemoryWeight(ar.Obs.TimeSerieRAM.PredictedValues.min, ar.Obs.TimeSerieRAM.lowest)))
					ar.Config["memoryAR"]["nmax"] = strconv.Itoa(wAverrage(ar.Obs.TimeSerieRAM.PredictedValues.max, generateMemoryWeight(ar.Obs.TimeSerieRAM.PredictedValues.max, ar.Obs.TimeSerieRAM.highest)))
					ar.Config["memoryAR"]["opti"] = strconv.Itoa(int(averrage(ar.Obs.TimeSerieRAM.PredictedValues.threshold)))
					ar.Config["memoryAR"]["usage"] = strconv.Itoa(int(input.Stats.MemoryStats.Usage))
				} else {
					memoryTurn++
				}

			} else if strings.Compare(category, "cpu%") == 0 && !ar.Obs.TimeSerieCPU.CPUPrediction {

				// The logic for the cpu loop is pretty much the same as memory, but more focused
				// on cpu cores

				// Generate CPU percent
				deltaUsage := float64(input.Stats.CPUStats.CPUUsage.TotalUsage) - float64(oldUsage)
				deltaSystem := float64(input.Stats.CPUStats.SystemUsage) - float64(oldSystem)
				numCPUs := float64(input.Stats.CPUStats.OnlineCPUs)
				CPUPercent := (deltaUsage / deltaSystem) * numCPUs * 100.0

				ar.Obs.TimeSerieCPU.percent = fifoFloat(ar.Obs.TimeSerieCPU.percent, CPUPercent, ar.Limit)
				ar.Obs.TimeSerieCPU.usage = fifoFloat(ar.Obs.TimeSerieCPU.usage, deltaUsage, ar.Limit)

				// Timeserie arrays are ready to be processed
				if cpuTurn > ar.Limit {
					cpuTurn = 0

					avPercent, avUsage := averrageF(ar.Obs.TimeSerieCPU.percent), averrageF(ar.Obs.TimeSerieCPU.usage)

					ar.Obs.TimeSerieCPU.PredictedValues.percent = fifoFloat(ar.Obs.TimeSerieCPU.PredictedValues.percent, avPercent, ar.Limit)
					ar.Obs.TimeSerieCPU.PredictedValues.usage = fifoFloat(ar.Obs.TimeSerieCPU.PredictedValues.usage, avUsage, ar.Limit)

					if len(ar.Obs.TimeSerieCPU.PredictedValues.percent) >= ar.Limit {
						cBestPercent := averrageF(ar.Obs.TimeSerieCPU.PredictedValues.percent)
						cBestUsage := averrageF(ar.Obs.TimeSerieCPU.PredictedValues.usage)

						// Display
						ar.Config["cpuAR"]["percentOpti"] = strconv.FormatFloat(cBestPercent, 'f', 3, 64)
						ar.Config["cpuAR"]["usageOpti"] = strconv.FormatFloat(cBestUsage, 'f', 0, 64)

						ar.Obs.TimeSerieCPU.CPUPrediction = true

					}
				} else {
					cpuTurn++
				}
				oldSystem, oldUsage = input.Stats.CPUStats.SystemUsage, input.Stats.CPUStats.CPUUsage.TotalUsage
			}
		}

		input.AutoRange = ConvertAutoRange(ar.Config)
		select {
		case ar.Output <- input:
		default:
		}
	}
}

func generateMemoryWeight(arr, h []uint64) []float32 {
	highest := highestOf(h)

	weight := make([]float32, 0, len(arr))

	for _, n := range arr {
		distance := float32((uint64(highest) / n))
		toAdd := 1 / distance
		if math.IsInf(float64(toAdd), 1) {
			toAdd = 1.0
		}
		weight = append(weight, toAdd)
	}
	return weight
}

func wAverrage(arr []uint64, weight []float32) int {
	var total int

	for i, n := range arr {
		total += int(float32(n) / weight[i])
	}
	return total / len(arr)
}

func averrageF(arr []float64) float64 {
	var total float64

	for _, n := range arr {
		total += n
	}
	return total / float64(len(arr))
}

func averrage(arr []uint64) uint64 {
	var total uint64

	for _, n := range arr {
		total += n
	}
	return total / uint64(len(arr))
}

func processMemoryStats(mstats types.MemoryStats, min, max, threshold int) (int, int) {
	usage := int(mstats.Usage)

	if usage > min+percent(max-min)*threshold {

		distance := percentageBetween(min, usage)

		min += distance * percent(min)
		max = min + threshold*percent(min)

	} else if usage < (min-percent(max-min))*threshold {

		min = usage + threshold*percent(usage)
		max = min + threshold*percent(min)

	}

	return min, max
}

// ConvertAutoRange is a function that is used to convert from swarm.AutoRange to types.AutoRange
func ConvertAutoRange(autoRange swarm.AutoRange) types.AutoRange {
	sl := make(types.AutoRange)
	for key := range autoRange {
		sl[key] = make(map[string]string)
		for subKey, subValue := range autoRange[key] {
			sl[key][subKey] = subValue
		}
	}
	return sl
}
