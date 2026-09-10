package root

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/martinghunt/lsfkit/internal/ostats"
	"github.com/spf13/cobra"
)

var ostatsOptions struct {
	outfile, timeUnits  string
	all, fails, summary bool
}
var ostatsCmd = &cobra.Command{
	Use:   "ostats [options] <output-files...>",
	Short: "Gather stats from finished LSF output files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out io.Writer
		if ostatsOptions.outfile == "-" {
			out = cmd.OutOrStdout()
		} else {
			f, e := os.Create(ostatsOptions.outfile)
			if e != nil {
				return e
			}
			defer f.Close()
			out = f
		}
		w := bufio.NewWriter(out)
		defer w.Flush()
		columns := ostats.ShortColumns
		if ostatsOptions.all {
			columns = ostats.AllColumns
		}
		counts := map[string]int{}
		noData := 0
		if !ostatsOptions.summary {
			fmt.Fprintln(w, strings.Join(append(columns, "filename"), "\t"))
		}
		for _, name := range args {
			records, e := ostats.ReadFile(name)
			if e != nil {
				return fmt.Errorf("read %q: %w", name, e)
			}
			if len(records) == 0 {
				noData++
				continue
			}
			for _, record := range records {
				if !ostats.HasData(record) {
					noData++
					continue
				}
				code := ostats.Row(record, []string{"exit_code"}, ostatsOptions.timeUnits)[0]
				counts[code]++
				if ostatsOptions.fails && code == "0" {
					continue
				}
				if !ostatsOptions.summary {
					fmt.Fprintln(w, strings.Join(append(ostats.Row(record, columns, ostatsOptions.timeUnits), name), "\t"))
				}
			}
		}
		if ostatsOptions.summary {
			fmt.Fprintln(w, "exit_code\tcount")
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "%s\t%d\n", k, counts[k])
			}
			if noData > 0 {
				fmt.Fprintf(w, "No_data\t%d\n", noData)
			}
		}
		return nil
	},
}

func init() {
	f := ostatsCmd.Flags()
	f.StringVarP(&ostatsOptions.outfile, "outfile", "o", "-", "output file (- for stdout)")
	f.StringVar(&ostatsOptions.timeUnits, "time-units", "h", "time units: s, m, or h")
	f.BoolVarP(&ostatsOptions.all, "all-columns", "a", false, "output all columns")
	f.BoolVarP(&ostatsOptions.fails, "fails", "f", false, "output only failed jobs")
	f.BoolVarP(&ostatsOptions.summary, "summary", "s", false, "summarize exit codes")
	f.Bool("longer", false, "deprecated alias for --all-columns")
	ostatsCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("longer") {
			ostatsOptions.all = true
		}
		if ostatsOptions.timeUnits != "s" && ostatsOptions.timeUnits != "m" && ostatsOptions.timeUnits != "h" {
			return fmt.Errorf("--time-units must be s, m, or h")
		}
		return nil
	}
}
