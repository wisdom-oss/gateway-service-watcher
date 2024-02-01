package globals

import "time"

// Environment contains the processed environment variables
var Environment map[string]string = make(map[string]string)

// ScanningInterval contains the parsed scanning interval from the environment
// or the default value of 1 minute
var ScanningInterval time.Duration
