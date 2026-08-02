package model

type Substation struct {
	ID  string  `json:"id"`
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Feeder struct {
	ID           string  `json:"id"`
	SubstationID string  `json:"substation_id"`
	Name         string  `json:"name"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
}

type Transformer struct {
	ID               string  `json:"id"`
	FeederID         string  `json:"feeder_id"`
	Lat              float64 `json:"lat"`
	Lon              float64 `json:"lon"`
	CapacityKVA      int     `json:"capacity_kva"`
	HouseholdsServed int     `json:"households_served"`
}

type GTPole struct {
	ID            string  `json:"id"`
	DTID          string  `json:"dt_id"`
	ParentPoleID  *string `json:"parent_pole_id"`
	SeqOnLine     int     `json:"seq_on_line"`
	IsBranchPoint bool    `json:"is_branch_point"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
}

type GTTopology struct {
	PoleID        string   `json:"pole_id"`
	ParentID      *string  `json:"parent_id"`
	DTID          string   `json:"dt_id"`
	SeqOnLine     int      `json:"seq_on_line"`
	Children      []string `json:"children"`
	IsBranchPoint bool     `json:"is_branch_point"`
	Lat           float64  `json:"lat"`
	Lon           float64  `json:"lon"`
}

type Pole struct {
	ID           string  `json:"id"`
	DTID         string  `json:"dt_id"`
	FeederID     string  `json:"feeder_id"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	SeqOnLine    *int    `json:"seq_on_line,omitempty"`
	ParentPoleID *string `json:"parent_pole_id,omitempty"`
	PoleType     string  `json:"pole_type"`
	Ward         string  `json:"ward"`
	Pincode      *string `json:"pincode,omitempty"`
	DeviceID     *string `json:"device_id,omitempty"`
}

type DTTopologyStatus struct {
	DTID        string `json:"dt_id"`
	HasTopology bool   `json:"has_topology"`
}
