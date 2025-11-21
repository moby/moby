// Code generated from OpenAPI definition. DO NOT EDIT.

package container

// TopResponse Container "top" response.
type TopResponse struct {
	// Each process running in the container, where each process
	// is an array of values corresponding to the titles.
	// Example: {
	//   "Processes": [
	//     [
	//       "root",
	//       "13642",
	//       "882",
	//       "0",
	//       "17:03",
	//       "pts/0",
	//       "00:00:00",
	//       "/bin/bash"
	//     ],
	//     [
	//       "root",
	//       "13735",
	//       "13642",
	//       "0",
	//       "17:06",
	//       "pts/0",
	//       "00:00:00",
	//       "sleep 10"
	//     ]
	//   ]
	// }
	Processes [][]string `json:"Processes,omitempty"`

	// The ps column titles
	// Example: {
	//   "Titles": [
	//     "UID",
	//     "PID",
	//     "PPID",
	//     "C",
	//     "STIME",
	//     "TTY",
	//     "TIME",
	//     "CMD"
	//   ]
	// }
	Titles []string `json:"Titles,omitempty"`
}
