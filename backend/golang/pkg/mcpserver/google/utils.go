package google

type TimeRange struct {
	From uint64 `json:"from" jsonschema:",description=The start timestamp in seconds of the time range, default is 0"`
	To   uint64 `json:"to"   jsonschema:",description=The end timestamp in seconds of the time range, default is 0"`
}
