package collectors

import (
	"regexp"
	"strconv"

	ps "github.com/mitchellh/go-ps"
	cg "github.com/phpHavok/cgroups_exporter/cgroups"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type cgroupsSlurmCollector struct {
	cpuacctUsagePerCPUMetric *prometheus.Desc
	cgroupsRootPath          string
}

func NewCgroupsSlurmCollector(cgroupsRootPath string) *cgroupsSlurmCollector {
	return &cgroupsSlurmCollector{
		cpuacctUsagePerCPUMetric: prometheus.NewDesc("cgroups_slurm_cpuacct_usage_per_cpu_ns",
			"Per-nanosecond usage of each CPU in a cgroup",
			[]string{"user_id", "job_id", "step_id", "task_id", "cpu_id"}, nil,
		),
		cgroupsRootPath: cgroupsRootPath,
	}
}

func (collector *cgroupsSlurmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.cpuacctUsagePerCPUMetric
}

func (collector *cgroupsSlurmCollector) Collect(ch chan<- prometheus.Metric) {
	// Get a list of all processes
	procs, err := ps.Processes()
	if err != nil {
		log.Fatalf("unable to read process table: %v", err)
	}
	// Filter processes by those running slurmstepd
	var slurmstepdIds []int
	for _, proc := range procs {
		if proc.Executable() == "slurmstepd" {
			slurmstepdIds = append(slurmstepdIds, proc.Pid())
		}
	}
	// Filter processes by children of slurmstepd processes
	for _, ssid := range slurmstepdIds {
		for _, proc := range procs {
			if proc.PPid() == ssid {
				cgroups, err := cg.LoadProcessCgroups(proc.Pid(), collector.cgroupsRootPath)
				if err != nil {
					log.Fatalf("unable to read cgroups file: %v", err)
				}
				slurmRegex := regexp.MustCompile(`/slurm(?:/uid_([^/]+))?(?:/job_([^/]+))?(?:/step_([^/]+))?(?:/task_([^/]+))?`)
				matches := slurmRegex.FindStringSubmatch(string(cgroups.Cpuacct))
				var (
					user_id string
					job_id  string
					step_id string
					task_id string
				)
				if len(matches) > 1 {
					user_id = matches[1]
				}
				if len(matches) > 2 {
					job_id = matches[2]
				}
				if len(matches) > 3 {
					step_id = matches[3]
				}
				if len(matches) > 4 {
					task_id = matches[4]
				}
				// cpuacctUsagePerCPUMetric
				usagePerCPU, err := cgroups.Cpuacct.GetUsagePerCPU()
				if err != nil {
					log.Fatalf("unable to read cpuacct usage per cpu: %v", err)
				}
				for cpuID, cpuUsage := range usagePerCPU {
					ch <- prometheus.MustNewConstMetric(collector.cpuacctUsagePerCPUMetric,
						prometheus.GaugeValue, float64(cpuUsage), user_id, job_id, step_id, task_id, strconv.Itoa(cpuID))
				}
			}
		}
	}
}
