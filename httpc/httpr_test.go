package httpc

import "testing"

func TestHttpr_Get(t *testing.T) {
	type info struct {
		IP     string `json:"ip" description:"IPv4地址"`
		Detail struct {
			IPv4     string `json:"IPv4" description:"IPv4地址"`
			IPv4Full string `json:"IPv4_full" description:"带端口的IPv4地址"`
			Ipv6     string `json:"IPv6" description:"IPv6地址"`
		} `json:"detail" description:"详细信息"`
	}

	// noinspection HttpUrlsUsage
	r, err := NewHttpr("tx.ifile.fun", "7290")
	if err != nil {
	}

	r.SetUrlPrefix("/api")

	resp := &info{}
	opt := r.Get("/ip", &Opt{ResponseModel: resp})

	if opt.IsOK() {
		t.Logf("get successfully: %s", resp.IP)
	} else {
		t.Error("get failed: ", opt.Err)
	}
}
