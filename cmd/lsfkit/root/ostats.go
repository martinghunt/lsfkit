package root

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/martinghunt/lsfkit/internal/ostats"
	"github.com/spf13/cobra"
)

type ostatsOptions struct {
	outfile       string
	timeUnit      string
	all           bool
	fails         bool
	includeNoData bool
	summary       bool
}

func newOstatsCommand() *cobra.Command {
	options := ostatsOptions{}
	command := &cobra.Command{
		Use:   "ostats [options] <output-files...>",
		Short: "Gather stats from finished LSF output files",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if options.timeUnit != "s" && options.timeUnit != "m" && options.timeUnit != "h" {
				return fmt.Errorf("--time-units must be s, m, or h")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, filenames []string) error {
			if options.outfile == "-" {
				return writeOstats(cmd.OutOrStdout(), filenames, options)
			}
			output, err := os.Create(options.outfile)
			if err != nil {
				return err
			}
			if err := writeOstats(output, filenames, options); err != nil {
				_ = output.Close()
				return err
			}
			return output.Close()
		},
	}

	flags := command.Flags()
	flags.StringVarP(&options.outfile, "outfile", "o", "-", "output file (- for stdout)")
	flags.StringVar(&options.timeUnit, "time-units", "h", "time units: s, m, or h")
	flags.BoolVarP(&options.all, "all-columns", "a", false, "output all columns")
	flags.BoolVarP(&options.fails, "fails", "f", false, "output only failed jobs")
	flags.BoolVar(&options.includeNoData, "include-no-data", false, "include a placeholder row for files without LSF job data")
	flags.BoolVarP(&options.summary, "summary", "s", false, "summarize exit codes")
	return command
}

func writeOstats(output io.Writer, filenames []string, options ostatsOptions) error {
	writer := bufio.NewWriter(output)
	columns := ostats.ShortColumns
	if options.all {
		columns = ostats.AllColumns
	}
	counts := map[string]int{}
	noData := 0
	if !options.summary {
		header := make([]string, 0, len(columns)+2)
		header = append(header, "number_in_file")
		header = append(header, columns...)
		header = append(header, "filename")
		fmt.Fprintln(writer, strings.Join(header, "\t"))
	}

	for _, filename := range filenames {
		records, err := ostats.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read %q: %w", filename, err)
		}
		if len(records) == 0 {
			noData++
			if options.includeNoData && !options.summary {
				writeNoDataRow(writer, columns, 1, filename, options.timeUnit)
			}
			continue
		}
		for number, record := range records {
			number++
			if !ostats.HasData(record) {
				noData++
				if options.includeNoData && !options.summary {
					writeNoDataRow(writer, columns, number, filename, options.timeUnit)
				}
				continue
			}
			code := ostats.Row(record, []string{"exit_code"}, options.timeUnit)[0]
			counts[code]++
			if options.fails && code == "0" {
				continue
			}
			if !options.summary {
				row := make([]string, 0, len(columns)+2)
				row = append(row, strconv.Itoa(number))
				row = append(row, ostats.Row(record, columns, options.timeUnit)...)
				row = append(row, filename)
				fmt.Fprintln(writer, strings.Join(row, "\t"))
			}
		}
	}

	if options.summary {
		fmt.Fprintln(writer, "exit_code\tcount")
		keys := make([]string, 0, len(counts))
		for code := range counts {
			keys = append(keys, code)
		}
		sort.Strings(keys)
		for _, code := range keys {
			fmt.Fprintf(writer, "%s\t%d\n", code, counts[code])
		}
		if noData > 0 {
			fmt.Fprintf(writer, "No_data\t%d\n", noData)
		}
	}
	return writer.Flush()
}

func writeNoDataRow(writer io.Writer, columns []string, number int, filename, timeUnit string) {
	row := make([]string, 0, len(columns)+2)
	row = append(row, strconv.Itoa(number))
	row = append(row, ostats.Row(ostats.Stats{}, columns, timeUnit)...)
	row = append(row, filename)
	fmt.Fprintln(writer, strings.Join(row, "\t"))
}
