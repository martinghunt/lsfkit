package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOstatsOutputFormats(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "jobs.o")
	contents := "Sender: LSF System <first>\nSuccessfully completed.\n    CPU time : 3600 sec.\n    Max Memory : 1500 MB\n" +
		"Sender: LSF System <second>\nExited with exit code 2.\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	wantTSV := "number_in_file\texit_code\tcpu_time\twall_clock_time\tmax_memory\trequested_memory\tfilename\n" +
		"1\t0\t1.00\t*\t1.5\t*\t" + filename + "\n" +
		"2\t2\t*\t*\t*\t*\t" + filename + "\n"
	wantTTSV := "filename\t" + filename + "\t" + filename + "\n" +
		"number_in_file\t1\t2\nexit_code\t0\t2\ncpu_time\t1.00\t*\nwall_clock_time\t*\t*\n" +
		"max_memory\t1.5\t*\nrequested_memory\t*\t*\n"
	for _, format := range []string{"", "table", "tsv", "ttable", "ttsv", "json"} {
		t.Run("format="+format, func(t *testing.T) {
			args := []string{"ostats", filename}
			if format != "" {
				args = append(args, "--outfmt", format)
			}
			output, err := executeCommand(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			switch format {
			case "json":
				var got []map[string]any
				if err := json.Unmarshal([]byte(output), &got); err != nil {
					t.Fatal(err)
				}
				want := []map[string]any{
					{"number_in_file": float64(1), "exit_code": float64(0), "cpu_time": float64(1), "wall_clock_time": nil, "max_memory": 1.5, "requested_memory": nil, "filename": filename},
					{"number_in_file": float64(2), "exit_code": float64(2), "cpu_time": nil, "wall_clock_time": nil, "max_memory": nil, "requested_memory": nil, "filename": filename},
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("JSON = %#v, want %#v", got, want)
				}
			case "tsv", "ttsv":
				want := wantTSV
				if format == "ttsv" {
					want = wantTTSV
				}
				if output != want {
					t.Fatalf("output = %q, want %q", output, want)
				}
			default:
				want := wantTSV
				if format == "ttable" {
					want = wantTTSV
				}
				if strings.Contains(output, "\t") || !strings.HasSuffix(output, "\n") {
					t.Fatalf("expected space-aligned table: %q", output)
				}
				gotLines, wantLines := strings.Split(output, "\n"), strings.Split(want, "\n")
				if len(gotLines) != len(wantLines) {
					t.Fatalf("unexpected row count: %q", output)
				}
				for i := range gotLines {
					if !reflect.DeepEqual(strings.Fields(gotLines[i]), strings.Fields(wantLines[i])) {
						t.Fatalf("row %d = %q, want %q", i, gotLines[i], wantLines[i])
					}
				}
			}
		})
	}
}

func TestOstatsSummaryFormats(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "jobs.o")
	contents := "Sender: LSF System <first>\nSuccessfully completed.\n" +
		"Sender: LSF System <second>\npartial\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for format, want := range map[string]string{
		"table":  "exit_code  count\n0          1\nNo_data    1\n",
		"tsv":    "exit_code\tcount\n0\t1\nNo_data\t1\n",
		"ttable": "exit_code  0  No_data\ncount      1  1\n",
		"ttsv":   "exit_code\t0\tNo_data\ncount\t1\t1\n",
		"json":   "[{\"exit_code\":0,\"count\":1},{\"exit_code\":\"No_data\",\"count\":1}]",
	} {
		t.Run(format, func(t *testing.T) {
			output, err := executeCommand(t, "ostats", "--summary", "--fails", "--outfmt", format, filename)
			if err != nil {
				t.Fatal(err)
			}
			if format == "json" {
				var gotValue, wantValue any
				if err := json.Unmarshal([]byte(output), &gotValue); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(gotValue, wantValue) {
					t.Fatalf("output = %s, want %s", output, want)
				}
			} else if output != want {
				t.Fatalf("output = %q, want %q", output, want)
			}
		})
	}
}

func TestOstatsJSONAllColumnsAndNoData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "jobs.o")
	contents := "Sender: LSF System <first>\nSuccessfully completed.\n" +
		"Sender: LSF System <second>\nJob <*> was submitted from host <submit> by user <me> in cluster <cluster>.\n" +
		"Started at Sun Sep 16 12:13:29 2013\nResults reported at Sun Sep 16 13:11:06 2013\n" +
		"Exited with exit code 2.\n    CPU time : 10864.48 sec.\n    Max Threads : 7\n" +
		"Sender: LSF System <third>\npartial\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := executeCommand(t, "ostats", "--outfmt", "json", "--all-columns", "--fails", "--include-no-data", "--time-units", "m", filename)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0]) != 15 || got[0]["number_in_file"] != float64(2) ||
		got[0]["job_name"] != "*" || got[0]["cpu_time"] != 181.07 || got[0]["wall_clock_time"] != 57.62 ||
		got[0]["max_threads"] != float64(7) || got[0]["start_time"] != "2013-09-16 12:13:29" ||
		got[1]["number_in_file"] != float64(3) || got[1]["exit_code"] != nil || got[1]["job_name"] != nil {
		t.Fatalf("unexpected JSON records: %s", output)
	}
}

func TestOstatsJSONEmpty(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "empty.o")
	if err := os.WriteFile(filename, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := executeCommand(t, "ostats", "--outfmt", "json", filename)
	if err != nil || output != "[]\n" {
		t.Fatalf("output = %q, error = %v", output, err)
	}
}

func TestOstatsRejectsBadFormatBeforeOpeningOutput(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "stats.txt")
	if err := os.WriteFile(filename, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := executeCommand(t, "ostats", "--outfmt", "csv", "--outfile", filename, "missing.o")
	if err == nil || !strings.Contains(err.Error(), "unsupported --outfmt") {
		t.Fatalf("unexpected error: %v", err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil || string(contents) != "keep me" {
		t.Fatalf("invalid format changed output file: %q, %v", contents, err)
	}
}

type failingOstatsWriter struct{}

func (failingOstatsWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestOstatsReportsWriteErrors(t *testing.T) {
	for _, format := range []string{"json", "table", "tsv", "ttable", "ttsv"} {
		err := writeOstats(failingOstatsWriter{}, nil, ostatsOptions{outfmt: format})
		if err == nil || err.Error() != "write failed" {
			t.Fatalf("%s: unexpected error: %v", format, err)
		}
	}
}

func TestOstatsTabularCells(t *testing.T) {
	var output bytes.Buffer
	err := writeOstatsRows(&output, [][]string{{"job_name", "filename"}, {"hello\tworld", "job\nname.o"}}, "tsv")
	if err != nil || output.String() != "job_name\tfilename\nhello world\tjob name.o\n" {
		t.Fatalf("output = %q, error = %v", output.String(), err)
	}
}
