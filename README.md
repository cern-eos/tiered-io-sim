# Tiered I/O Simulator

A web app for sizing an NVMe cache, an HDD tier, and a tape carousel together. It answers whether tape can take archive and recall traffic so the HDD tier can shrink below a 2.5 EB baseline and still serve the compute workload.

![Tiered I/O Simulator, default 1.5 EB HDD layout](docs/screenshot.png)

## Run

```
go run .
```

The simulator listens on port 8090 on all interfaces. Open [http://127.0.0.1:8090](http://127.0.0.1:8090) on this machine. Pass `-addr` to use another address.

## What it models

**NVMe and HDD.** Nodes, a 100/400/800 GbE network, drives per node, drive size, sequential bandwidth, and IOPS. Delivered bandwidth is the minimum of aggregate drive bandwidth and the node network. Network GB/s is the line rate divided by 8.

**HDD streams.** Active read and write streams share each drive. Their sum divided by the drive count is *n* in

```
BW(n) = n·S / (n·(S / BW_seq) + (n − 1)·t_seek)
```

for more than one stream per drive. Below that, seeks are not charged. The chart holds the read/write mix and sweeps *n*, with bandwidth on the left axis and the fraction of sequential bandwidth lost to seeks on the right.

**Tape.** Bandwidth is the drive count times bandwidth per drive. Library capacity is a separate input and does not follow from the drive count.

**Workload.** Compute read and write, an NVMe hit rate, working set, reserve, archive in EB/year, recall, and an observed TB/s-per-EB reference. The size sweep changes only the HDD node count and reports bandwidth, slack, and cost at each size.

**Cost.** A node price for every NVMe and HDD server, media in CHF/TB, a price per tape drive, and tape media in CHF/TB. Each tier is split into servers and media.

Units are decimal: 1 EB = 1,000,000 TB. A year is 365 days.

## Tests

```
go test ./...
```
