package perftest

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ProcessCount int
	TestBin      string
	MsgSize      int
	NCCLExtraEnv []string
	Timeout      time.Duration
}

func buildCmdConfig(processCount int, msgSize int) Config {
	return Config{
		ProcessCount: processCount,
		TestBin:      "/usr/local/sihpc/libexec/nccl-tests/all_reduce_perf",
		MsgSize:      msgSize,
		Timeout:      10 * time.Minute,
		NCCLExtraEnv: []string{
			"NCCL_SOCKET_IFNAME=bond0",
			"SHARP_COLL_ENABLE_PCI_RELAXED_ORDERING=1",
			"NCCL_COLLENT_ENABLE=0",
			"NCCL_NVLS_ENABLE=0",
		},
	}
}

func buildMpiCmd(cfg Config) *exec.Cmd {
	args := []string{
		"-np", fmt.Sprint(cfg.ProcessCount),
		// "--map-by", fmt.Sprintf("ppr:%d:node", cfg.ProcessCount),
		"--bind-to", "numa",
		"-mca", "coll_hcoll_enable", "0",
		"--allow-run-as-root",
	}

	for _, ev := range cfg.NCCLExtraEnv {
		args = append(args, "-x", ev)
	}

	// nccl-tests
	testLine := []string{
		cfg.TestBin,
		// "-b", cfg.MsgSize,
		// "-e", cfg.MsgSize,
		// "-f", "2",
		// "-g", "1",
	}
	args = append(args, testLine...)

	cmd := exec.Command("mpirun", args...)
	return cmd
}

type Record struct {
	MsgSize string
	AlgBW   string
}

func runNcclTest(cfg Config) ([]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	cmd := buildMpiCmd(cfg)

	fmt.Println("Command:", cmd.String())
	cmdCtx := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	output, err := cmdCtx.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("excute error :%v", err)
	}

	var res []float64
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Avg bus bandwidth") {
			bwStr := strings.TrimSpace(strings.Split(line, ":")[1])
			bwStr = strings.Split(bwStr, " ")[0]
			bw, err := strconv.ParseFloat(bwStr, 64)
			if err != nil {
				return res, fmt.Errorf("convert err: %v", err)
			}
			res = append(res, bw)
		}
	}
	return res, nil
}

func checkBandwidth(avgBusBandwidths []float64, exceptBwGbps float64) int {
	var sum float64
	for _, bw := range avgBusBandwidths {
		sum += bw
	}
	avgBusBandwidth := sum / float64(len(avgBusBandwidths))

	if avgBusBandwidth < exceptBwGbps {
		fmt.Printf("NCCL allreduce bandwidth test failed, avgBusBandwidth returned %.2f Gbps, but expected > %.2f Gbps.\n", avgBusBandwidth, exceptBwGbps)
		return 1
	} else {
		fmt.Printf("NCCL allreduce bandwidth test passed, avgBusBandwidth = %.2f Gbps.\n", avgBusBandwidth)
		return 0
	}
}

func CheckNcclPerf(processCount int, msgSize int, exceptBwGbps float64) error {
	job := buildCmdConfig(processCount, msgSize)

	records, err := runNcclTest(job)
	if err != nil {
		return fmt.Errorf("run fail: %v", err)

	}
	for _, record := range records {
		fmt.Printf("record: %v\n", record)
	}
	if len(records) == 0 {
		return fmt.Errorf("get no avg bus bandwidth res")
	}
	checkBandwidth(records, exceptBwGbps)

	return nil
}
