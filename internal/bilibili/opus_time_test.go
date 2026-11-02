package bilibili

import (
	"fmt"
	"testing"
)

func TestOpusPublicationFormats(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{{"number", "1700000000", true}, {"string", `"1700000000"`, true}, {"fraction", `"1.5"`, false}, {"overflow", `"999999999999999999999"`, false}, {"negative", "-1", false}, {"text", `"bad"`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseOpus([]byte(fmt.Sprintf(`{"code":0,"data":{"item":{"id_str":"123","modules":[{"module_author":{"mid":42,"pub_ts":%s}}]}}}`, tc.raw)))
			if tc.valid {
				if err != nil || out.PublishedAt != 1700000000 {
					t.Fatalf("publication: %+v %v", out, err)
				}
			} else if err == nil {
				t.Fatal("invalid timestamp accepted")
			}
		})
	}
}
