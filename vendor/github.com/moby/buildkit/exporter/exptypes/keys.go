package exptypes

const (
	ExporterEpochKey = "source.date.epoch"
)

type ExporterOptKey string

// Options keys supported by all exporters.
var (
	// Clamp produced timestamps. For more information see the
	// SOURCE_DATE_EPOCH specification.
	// Value: int (number of seconds since Unix epoch)
	OptKeySourceDateEpoch ExporterOptKey = "source-date-epoch"
)
