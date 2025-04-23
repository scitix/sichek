package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	NRanks       int      // 总进程数（=总 GPU 数）
	GPUsPerNode  int      // 每节点 GPU 数，用于 map-by ppr
	TestBin      string   // nccl-tests 可执行文件
	MsgBegin     string   // -b 参数，比如 "8"
	MsgEnd       string   // -e 参数，比如 "4G"
	DtypeFlag    string   // -f 参数，"2"=fp16, "4"=fp32
	NCCLExtraEnv []string // 其他想透传的 NCCL_* 环境变量
	Timeout      time.Duration
}

func defaultJob() Config {
	return Config{
		NRanks:      8,
		GPUsPerNode: 4,
		TestBin:     "./build/all_reduce_perf",
		MsgBegin:    "8",
		MsgEnd:      "4",
		DtypeFlag:   "2",
		Timeout:     10 * time.Minute,
		NCCLExtraEnv: []string{
			"NCCL_DEBUG=INFO",
			"NCCL_SOCKET_IFNAME=ib0",
			"NCCL_IB_GID_INDEX=3",
		},
	}
}

func buildMpiCmd(cfg Config) *exec.Cmd {
	args := []string{
		"-np", fmt.Sprint(cfg.NRanks),
		"--map-by", fmt.Sprintf("ppr:%d:node", cfg.GPUsPerNode),
		"--bind-to", "numa",
		"-mca", "coll_hcoll_enable", "0",
		"--allow-run-as-root",
	}

	for _, ev := range append(cfg.NCCLExtraEnv, "LD_LIBRARY_PATH") {
		args = append(args, "-x", ev)
	}

	// nccl-tests
	testLine := []string{
		cfg.TestBin,
		"-b", cfg.MsgBegin,
		"-e", cfg.MsgEnd,
		"-f", cfg.DtypeFlag,
		"-g", "1",
	}
	args = append(args, testLine...)

	cmd := exec.Command("mpirun", args...)
	return cmd
}

var bwRe = regexp.MustCompile(`AlgBW\s+([0-9.]+)\s*GB/s`)
var sizeRe = regexp.MustCompile(`^\s*([0-9.]+[KMGT]?)\s+`)

type Record struct {
	MsgSize string
	AlgBW   string
}

func runNcclTest(cfg Config) ([]Record, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	cmd := buildMpiCmd(cfg)
	cmdCtx := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	pipe, _ := cmdCtx.StdoutPipe()
	cmdCtx.Stderr = cmdCtx.Stdout

	if err := cmdCtx.Start(); err != nil {
		return nil, err
	}

	var rec []Record
	sc := bufio.NewScanner(pipe)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "#") {
			if m := bwRe.FindStringSubmatch(line); m != nil {
				bw := m[1]
				sz := "?"
				if ms := sizeRe.FindStringSubmatch(line); ms != nil {
					sz = ms[1]
				}
				rec = append(rec, Record{MsgSize: sz, AlgBW: bw})
			}
		}
		fmt.Println(line)
	}
	if err := cmdCtx.Wait(); err != nil {
		return nil, err
	}
	return rec, sc.Err()
}

func CHeckNcclPerf() {
	job := defaultJob()

	records, err := runNcclTest(job)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run fail: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------
func saveCSV(path string, rec []Record) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Println("csv create:", err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"MessageSize", "AlgBW(GB/s)"})
	for _, r := range rec {
		w.Write([]string{r.MsgSize, r.AlgBW})
	}
	fmt.Println("result saved to", path)
}

func nodesInFile(file string) int {
	b, err := os.ReadFile(file)
	if err != nil {
		return 1
	}
	lines := 0
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			lines++
		}
	}
	return lines
}
