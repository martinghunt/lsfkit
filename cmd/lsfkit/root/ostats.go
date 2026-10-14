package root

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/martinghunt/lsfkit/internal/ostats"
	"github.com/spf13/cobra"
)

type ostatsOptions struct {
	outfile       string
	outfmt        string
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
			options.outfmt = strings.ToLower(strings.TrimSpace(options.outfmt))
			switch options.outfmt {
			case "json", "table", "tsv", "ttable", "ttsv":
			default:
				return fmt.Errorf("unsupported --outfmt %q; expected json, table, tsv, ttable, or ttsv", options.outfmt)
			}
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
	flags.StringVar(&options.outfmt, "outfmt", "table", "output format: json, table, tsv, ttable, or ttsv")
	flags.StringVar(&options.timeUnit, "time-units", "h", "time units: s, m, or h")
	flags.BoolVarP(&options.all, "all-columns", "a", false, "output all columns")
	flags.BoolVarP(&options.fails, "fails", "f", false, "output only failed jobs (ignored with --summary)")
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
	var rows [][]string
	jsonRows := make([]map[string]any, 0)
	addRecord := func(record ostats.Stats, number int, filename string) {
		if options.outfmt == "json" {
			jsonRows = append(jsonRows, ostatsJSONRow(record, columns, number, filename, options.timeUnit))
			return
		}
		row := []string{strconv.Itoa(number)}
		row = append(row, ostats.Row(record, columns, options.timeUnit)...)
		rows = append(rows, append(row, filename))
	}
	if !options.summary {
		header := make([]string, 0, len(columns)+2)
		header = append(header, "number_in_file")
		header = append(header, columns...)
		header = append(header, "filename")
		rows = append(rows, header)
	}

	for _, filename := range filenames {
		records, err := ostats.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read %q: %w", filename, err)
		}
		if len(records) == 0 {
			noData++
			if options.includeNoData && !options.summary {
				addRecord(ostats.Stats{}, 1, filename)
			}
			continue
		}
		for number, record := range records {
			number++
			if !ostats.HasData(record) {
				noData++
				if options.includeNoData && !options.summary {
					addRecord(ostats.Stats{}, number, filename)
				}
				continue
			}
			code := ostats.Row(record, []string{"exit_code"}, options.timeUnit)[0]
			counts[code]++
			if options.fails && code == "0" {
				continue
			}
			if !options.summary {
				addRecord(record, number, filename)
			}
		}
	}

	if options.summary {
		rows = [][]string{{"exit_code", "count"}}
		keys := make([]string, 0, len(counts))
		for code := range counts {
			keys = append(keys, code)
		}
		sort.Strings(keys)
		for _, code := range keys {
			rows = append(rows, []string{code, strconv.Itoa(counts[code])})
			var exitCode any
			if code != "*" {
				exitCode, _ = strconv.Atoi(code)
			}
			jsonRows = append(jsonRows, map[string]any{"exit_code": exitCode, "count": counts[code]})
		}
		if noData > 0 {
			rows = append(rows, []string{"No_data", strconv.Itoa(noData)})
			jsonRows = append(jsonRows, map[string]any{"exit_code": "No_data", "count": noData})
		}
	}
	if options.outfmt == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(jsonRows); err != nil {
			return err
		}
	} else if err := writeOstatsRows(writer, rows, options.outfmt); err != nil {
		return err
	}
	return writer.Flush()
}

func ostatsJSONRow(record ostats.Stats, columns []string, number int, filename, unit string) map[string]any {
	timeValue := func(value *float64) any {
		if value == nil {
			return nil
		}
		divisor := 1.0
		if unit == "m" {
			divisor = 60
		} else if unit == "h" {
			divisor = 3600
		}
		return math.Round(*value*(1/divisor)*100) / 100
	}
	timestamp := func(value *time.Time) any {
		if value == nil {
			return nil
		}
		return value.Format("2006-01-02 15:04:05")
	}
	values := map[string]any{
		"exit_code": record.ExitCode, "cpu_time": timeValue(record.CPUTime),
		"wall_clock_time": timeValue(record.WallClockTime),
		"max_memory":      record.MaxMemory, "requested_memory": record.RequestedMemory,
		"max_processes": record.MaxProcesses, "max_threads": record.MaxThreads,
		"start_time": timestamp(record.StartTime), "end_time": timestamp(record.EndTime),
		"exec_host": record.ExecHost, "username": record.Username,
		"working_dir": record.WorkingDir, "job_name": record.JobName,
	}
	row := map[string]any{"number_in_file": number, "filename": filename}
	for _, column := range columns {
		row[column] = values[column]
	}
	return row
}

func writeOstatsRows(writer io.Writer, rows [][]string, format string) error {
	if len(rows) == 0 {
		return nil
	}
	if format == "ttable" || format == "ttsv" {
		transposed := make([][]string, len(rows[0]))
		for column := range transposed {
			for _, row := range rows {
				transposed[column] = append(transposed[column], row[column])
			}
		}
		rows = transposed
		for index, row := range rows {
			if row[0] == "filename" {
				copy(rows[1:index+1], rows[:index])
				rows[0] = row
				break
			}
		}
	}
	widths := make([]int, len(rows[0]))
	replacer := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	for _, row := range rows {
		for column, value := range row {
			// Keep tabs and line breaks in paths and job names within one cell.
			row[column] = replacer.Replace(value)
			widths[column] = max(widths[column], utf8.RuneCountInString(row[column]))
		}
	}
	for _, row := range rows {
		line := strings.Join(row, "\t")
		if format == "table" || format == "ttable" || format == "" {
			var cells []string
			for column, value := range row {
				if column < len(row)-1 {
					value += strings.Repeat(" ", widths[column]-utf8.RuneCountInString(value))
				}
				cells = append(cells, value)
			}
			line = strings.Join(cells, "  ")
		}
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	return nil
}
