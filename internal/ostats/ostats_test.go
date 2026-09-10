package ostats

import (
	"math"
	"strings"
	"testing"
)

const notification = `Sender: LSF System <lsfadmin@host>
Job <a very long job name> was submitted from host <submit> by user <me> in cluster <cluster>.
Job was executed on host(s) <node1>, in queue <normal>, as user <me> in cluster <cluster>.
</work> was used as the working directory.
Started at Sun Sep 16 12:13:29 2013
Results reported at Sun Sep 16 13:11:06 2013
Successfully completed.
    CPU time :               10864.48 sec.
    Max Memory :             1184 MB
    Total Requested Memory : 2000.00 MB
    Max Processes :          6
    Max Threads :            7
`

func TestReadAtEOFWithoutFooter(t *testing.T) {
	records, e := Read(strings.NewReader(notification))
	if e != nil {
		t.Fatal(e)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records", len(records))
	}
	r := records[0]
	if r.JobName == nil || *r.JobName != "a very long job name" {
		t.Fatal(r.JobName)
	}
	if r.CPUTime == nil || math.Abs(*r.CPUTime-10864.48) > 1e-9 ||
		r.WallClockTime == nil || *r.WallClockTime != 3457 {
		t.Fatalf("unexpected parsed times: %#v", r)
	}
}

func TestReadMultipleRecords(t *testing.T) {
	records, e := Read(strings.NewReader(notification + notification))
	if e != nil || len(records) != 2 {
		t.Fatalf("records=%d err=%v", len(records), e)
	}
}

func TestTimeColumnsAreRoundedToTwoDecimalPlaces(t *testing.T) {
	records, err := Read(strings.NewReader(notification))
	if err != nil {
		t.Fatal(err)
	}
	got := Row(records[0], ShortColumns, "h")
	if got[1] != "3.02" || got[2] != "0.96" {
		t.Fatalf("unexpected rounded time values: %v", got)
	}
}

func FuzzReadNeverHangs(f *testing.F) {
	f.Add(notification)
	f.Add("Sender: LSF System <x>\npartial")
	f.Fuzz(func(t *testing.T, input string) { _, _ = Read(strings.NewReader(input)) })
}
