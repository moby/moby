Display a live stream of one or more containers' resource usage statistics

# Format

   Pretty-print containers statistics using a Go template.
   Valid placeholders:
      .Container - Container name or ID.
      .Name - Container name.
      .ID - Container ID.
      .CPUPerc - CPU percentage.
      .MemUsage - Memory usage.
      .NetIO - Network IO.
      .BlockIO - Block IO.
      .MemPerc - Memory percentage (Not available on Windows).
      .PIDs - Number of PIDs (Not available on Windows).

# EXAMPLES

Running `docker container stats` on all running containers.

    $ docker container stats
    CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
    1285939c1fd3        0.07%               796 KiB / 64 MiB        1.21%               788 B / 648 B       3.568 MB / 512 KB
    9c76f7834ae2        0.07%               2.746 MiB / 64 MiB      4.29%               1.266 KB / 648 B    12.4 MB / 0 B
    d1ea048f04e4        0.03%               4.583 MiB / 64 MiB      6.30%               2.854 KB / 648 B    27.7 MB / 0 B

Running `docker container stats` on multiple containers by name and id.

    $ docker container stats fervent_panini 5acfcb1b4fd1
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    5acfcb1b4fd1        0.00%               115.2 MiB/1.045 GiB   11.03%              1.422 kB/648 B
    fervent_panini      0.02%               11.08 MiB/1.045 GiB   1.06%               648 B/648 B

Running `docker container stats` with customized format on all (Running and Stopped) containers.

    $ docker container stats --all --format "table {{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
    CONTAINER ID                                                       NAME                     CPU %               MEM USAGE / LIMIT
    c9dfa83f0317f87637d5b7e67aa4223337d947215c5a9947e697e4f7d3e0f834   ecstatic_noether         0.00%               56KiB / 15.57GiB
    8f92d01cf3b29b4f5fca4cd33d907e05def7af5a3684711b20a2369d211ec67f   stoic_goodall            0.07%               32.86MiB / 15.57GiB
    38dd23dba00f307d53d040c1d18a91361bbdcccbf592315927d56cf13d8b7343   drunk_visvesvaraya       0.00%               0B / 0B
    5a8b07ec4cc52823f3cbfdb964018623c1ba307bce2c057ccdbde5f4f6990833   big_heisenberg           0.00%               0B / 0B

`drunk_visvesvaraya` and `big_heisenberg` are stopped containers in the above example.
