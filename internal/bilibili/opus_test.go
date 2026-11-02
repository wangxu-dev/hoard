package bilibili

import "testing"

func TestOpusParsing(t *testing.T) {
	works, more, err := ParseOpusPage([]byte(`{"code":0,"data":{"has_more":false,"items":[{"opus_id":"123","content":"Example","cover":{"url":"http://i0.hdslb.com/example.jpg"}}]}}`))
	if err != nil || more || len(works) != 1 || works[0].Kind != "gallery" || works[0].RemoteID != "opus:123" || works[0].CoverURL != "https://i0.hdslb.com/example.jpg" {
		t.Fatalf("page: %+v %v", works, err)
	}
	opus, err := ParseOpus([]byte(`{"code":0,"data":{"item":{"id_str":"123","basic":{"title":"Title"},"modules":[{"module_content":{"paragraphs":[{"text":{"nodes":[{"word":{"words":"Body"}}]}},{"pic":{"pics":[{"url":"https://i0.hdslb.com/example.jpg"},{"url":"https://evil.example/private"}]}}]}}]}}}`))
	if err != nil || opus.Text != "Title\nBody" || len(opus.Images) != 1 {
		t.Fatalf("detail: %+v %v", opus, err)
	}
	for _, body := range []string{`{"code":0,"data":{"has_more":true,"items":[]}}`, `{"code":0,"data":{"items":[]}}`, `{"code":0,"data":{"has_more":false,"items":[{"opus_id":"bad"}]}}`} {
		if _, _, err := ParseOpusPage([]byte(body)); err == nil {
			t.Fatal("invalid page accepted")
		}
	}
}
