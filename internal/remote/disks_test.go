package remote

import "testing"

const diskSample = "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/root 10485760 4194304 5767168 43% /\n/dev/mapper/data 2097152 1048576 1048576 50% /srv/shared files\ntmpfs 1024 0 1024 0% /run\n"

func TestDiskUsagePreservesReportedMetrics(t *testing.T) {
	rows, err := ParseDiskUsage(diskSample)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].TotalKiB != 10485760 || rows[0].UsedKiB != 4194304 || rows[0].AvailableKiB != 5767168 || rows[0].Percent != 43 {
		t.Fatal(rows)
	}
	if rows[1].Mount != "/srv/shared files" || rows[2].UsedKiB != 0 {
		t.Fatal(rows)
	}
	full, err := ParseDiskUsage("Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/full 100 101 -1 101% /full\n")
	if err != nil || full[0].AvailableKiB != -1 || full[0].Percent != 101 {
		t.Fatal(full, err)
	}
}
func TestDiskUsageRejectsBrokenRows(t *testing.T) {
	for _, sample := range []string{"", "permission denied", "Filesystem blocks Used Available Use% Mounted on\n/dev/sda invalid 10 20 30% /\n", "Filesystem blocks Used Available Use% Mounted on\n/dev/sda 99999999999999999999999999 10 20 30% /\n"} {
		if _, err := ParseDiskUsage(sample); err == nil {
			t.Fatal("invalid sample accepted", sample)
		}
	}
}
