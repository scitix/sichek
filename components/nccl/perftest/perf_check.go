package perftest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type Config struct {
	NumGpus int
	TestBin string
	UseNvls bool
}

func buildCmdConfig(numGpus int, useNvls bool) Config {
	return Config{
		NumGpus: numGpus,
		TestBin: "nccl_perf",
		UseNvls: useNvls,
	}
}

func GetDefaultNcclTestPath(testBin string) (string, error) {
	defaultCfgDirPath := filepath.Join(consts.DefaultPodCfgPath, testBin)
	_, err := os.Stat(defaultCfgDirPath)
	if err != nil {
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", fmt.Errorf("get curr file path failed")
		}
		defaultCfgDirPath = filepath.Join(filepath.Dir(curFile), testBin)
	}
	return defaultCfgDirPath, nil
}

func buildNcclTestCmd(cfg Config) *exec.Cmd {
	// nccl-tests
	testPath, err := GetDefaultNcclTestPath(cfg.TestBin)
	if err != nil {
		logrus.WithField("perftest", "nccl").Errorf("GetDefaultNcclTestPath error :%v\n", err)
		return nil
	}
	args := []string{
		testPath,
	}
	if cfg.NumGpus != 0 {
		args = append(args, fmt.Sprintf("-g %d", cfg.NumGpus))
	}
	fmt.Printf("== Run %d GPU nccl all_reduce test ==\n", cfg.NumGpus)
	cmd := exec.Command("bash", args...)
	if !cfg.UseNvls {
		cmd.Env = append(os.Environ(), "NCCL_NVLS_ENABLE=0")
	}
	return cmd
}

func runNcclTest(cfg Config) ([]float64, error) {
	cmd := buildNcclTestCmd(cfg)
	logrus.WithField("perftest", "nccl").Infof("Command: %s\n", cmd.String())
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	logrus.WithField("perftest", "nccl").Infof("output: %s\n", outputStr)
	if err != nil {
		return nil, err
	}

	var res []float64
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Avg bus bandwidth") {
			bwStr := strings.TrimSpace(strings.Split(line, ":")[1])
			bwStr = strings.Split(bwStr, " ")[0]
			bw, err := strconv.ParseFloat(bwStr, 64)
			if err != nil {
				return nil, fmt.Errorf("parse bandwidth err: %v", err)
			}
			res = append(res, bw)
		}
	}
	return res, nil
}

func checkBandwidth(avgBusBandwidths []float64, exceptBwGbps float64) *common.Result {
	var sum float64

	resItem := NcclPerfCheckItems[NcclPerfCheckerName]
	for _, bw := range avgBusBandwidths {
		sum += bw
	}
	avgBusBandwidth := sum / float64(len(avgBusBandwidths))

	if avgBusBandwidth < exceptBwGbps {
		resItem.Detail = fmt.Sprintf("NCCL allreduce bandwidth test failed, avgBusBandwidth returned %.2f Gbps, but expected > %.2f Gbps.\n", avgBusBandwidth, exceptBwGbps)

	} else {
		resItem.Status = consts.StatusAbnormal
		resItem.Detail = fmt.Sprintf("NCCL allreduce bandwidth test passed, avgBusBandwidth = %.2f Gbps.\n", avgBusBandwidth)
	}
	res := &common.Result{
		Item:     "NcclPerf",
		Status:   resItem.Status,
		Level:    resItem.Level,
		Checkers: []*common.CheckerResult{resItem},
	}
	return res

}

func CheckNcclPerf(numGpus int, enableNvls bool, exceptBwGbps float64) (*common.Result, error) {
	job := buildCmdConfig(numGpus, enableNvls)
	records, err := runNcclTest(job)
	if err != nil {
		return nil, fmt.Errorf("run nccl test fail: %v", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("get no avg bus bandwidth res")
	}
	res := checkBandwidth(records, exceptBwGbps)

	return res, nil
}

func PrintInfo(result *common.Result) bool {
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			fmt.Printf("%s%s%s\n", consts.Red, result.Detail, consts.Reset)
			return false
		} else {
			fmt.Printf("%s%s%s\n", consts.Green, result.Detail, consts.Reset)
			return true
		}
	}
	return true
}
