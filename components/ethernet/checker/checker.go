package checker

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/sirupsen/logrus"
)

func NewCheckers(cfg *config.EthernetUserConfig, spec *config.EthernetSpec) ([]common.Checker, error) {
	checkerConstructors := map[string]func(*config.EthernetSpec) (common.Checker, error){
		config.ChekEthPhyState: NewEthPhyStateChecker,
	}

	ignoredSet := make(map[string]struct{})
	for _, checker := range cfg.Ethernet.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}
	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)
	for checkerName := range config.EthCheckItems {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		if constructor, exists := checkerConstructors[checkerName]; exists {
			checker, err := constructor(spec)
			if err != nil {
				logrus.WithError(err).WithField("checker", checkerName).Error("Failed to create checker")
				continue
			}
			usedCheckers = append(usedCheckers, checker)
			usedCheckersName = append(usedCheckersName, checkerName)
		}
	}
	logrus.WithField("component", "Ethernet-Checker").Infof("usedCheckersName: %v, ignoredCheckers: %v", usedCheckersName, cfg.Ethernet.IgnoredCheckers)

	return usedCheckers, nil
}
