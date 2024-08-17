package sentencepiece

import (
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"
)

func BenchmarkEncoder(b *testing.B) {
	buf, err := ioutil.ReadFile(filepath.Join("test", "pg7193_english.txt"))
	if err != nil {
		b.Fatal(err)
	}
	sbuf := string(buf)

	proc := createProcessor(b)
	b.ResetTimer()
	total := 0

	for i := 0; i < b.N; i++ {
		toks := proc.Encode(sbuf)
		total += len(toks)
	}
	runtime.KeepAlive(total)

	b.ReportMetric(float64(total)/float64(b.Elapsed().Seconds()), "tokens/sec")
}

func BenchmarkDecoder(b *testing.B) {
	buf, err := ioutil.ReadFile(filepath.Join("test", "pg7193_english.txt"))
	if err != nil {
		b.Fatal(err)
	}
	sbuf := string(buf)

	proc := createProcessor(b)
	toks := proc.Encode(sbuf)

	b.ResetTimer()
	total := 0

	for i := 0; i < b.N; i++ {
		t := proc.DecodeTokens(toks)
		total += len(t)
	}
	runtime.KeepAlive(total)

	b.ReportMetric(float64(len(toks)*b.N)/float64(b.Elapsed().Seconds()), "tokens/sec")
}
