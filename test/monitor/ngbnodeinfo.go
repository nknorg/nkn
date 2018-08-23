package monitor

type NgbNodeInfo struct {
	NgbId         string
	NgbType       string
	NgbAddr       string
	HttpInfoAddr  string
	HttpInfoPort  uint16
	HttpInfoStart bool
}

type NgbNodeInfoSlice []NgbNodeInfo

func (n NgbNodeInfoSlice) Len() int {
	return len(n)
}

func (n NgbNodeInfoSlice) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n NgbNodeInfoSlice) Less(i, j int) bool {
	return n[i].HttpInfoAddr < n[j].HttpInfoAddr
}
