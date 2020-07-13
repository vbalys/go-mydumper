[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Build/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Build%22+event%3Apush)
[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Test/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Test%22+event%3Apush)
[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Coverage/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Coverage%22+event%3Apush)
[![Go Report Card](https://goreportcard.com/badge/github.com/xelabs/go-mydumper)](https://goreportcard.com/report/github.com/xelabs/go-mydumper) [![codecov.io](https://codecov.io/gh/xelabs/go-mydumper/graphs/badge.svg)](https://codecov.io/gh/xelabs/go-mydumper/branch/master)

# go-mydumper

***go-mydumper*** is a multi-threaded MySQL backup and restore tool, and it is compatible with [maxbube/mydumper](https://github.com/maxbube/mydumper) in the layout.


## Build

```
$ git clone https://github.com/aquarapid/go-mydumper
$ cd go-mydumper
$ git checkout jacques_vitess
$ make build
$ ./bin/mydumper -h
$ ./bin/myloader -h
```

## Usage

### mydumper

```
./bin/mydumper -h
Usage: ./bin/mydumper -c conf/mydumper.ini.vitess
  -c string
    	config file

Examples:
$ ./bin/mydumper -c conf/mydumper.ini.vitess 
 2020/07/13 11:29:55.514476 dumper.go:33:        [INFO]         dumping.database[commerce].schema...
 2020/07/13 11:29:55.555823 dumper.go:43:        [INFO]         dumping.table[commerce.corder].schema...
 2020/07/13 11:29:55.555970 dumper.go:235:       [INFO]         dumping.table[commerce.corder].datas.thread[0]...
 2020/07/13 11:29:55.559696 dumper.go:43:        [INFO]         dumping.table[commerce.customer].schema...
 2020/07/13 11:29:55.560089 dumper.go:235:       [INFO]         dumping.table[commerce.customer].datas.thread[1]...
 2020/07/13 11:29:55.563361 dumper.go:43:        [INFO]         dumping.table[commerce.product].schema...
 2020/07/13 11:29:55.563573 dumper.go:235:       [INFO]         dumping.table[commerce.product].datas.thread[2]...
 2020/07/13 11:29:55.565112 dumper.go:142:       [INFO]         dumping.table[commerce.corder].done.allrows[1].allbytes[0MB].thread[0]...
 2020/07/13 11:29:55.565183 dumper.go:237:       [INFO]         dumping.table[commerce.corder].datas.thread[0].done...
 2020/07/13 11:29:55.568321 dumper.go:142:       [INFO]         dumping.table[commerce.customer].done.allrows[0].allbytes[0MB].thread[1]...
 2020/07/13 11:29:55.568356 dumper.go:237:       [INFO]         dumping.table[commerce.customer].datas.thread[1].done...
 2020/07/13 11:29:55.570556 dumper.go:142:       [INFO]         dumping.table[commerce.product].done.allrows[1].allbytes[0MB].thread[2]...
 2020/07/13 11:29:55.570598 dumper.go:237:       [INFO]         dumping.table[commerce.product].datas.thread[2].done...
 2020/07/13 11:29:55.570646 dumper.go:256:       [INFO]         dumping.all.done.cost[0.06sec].allrows[2].allbytes[41].rate[0.00MB/s]
```

The dump files:
```
ls ./dumper-sql/
commerce.corder.00001.sql  commerce.corder-schema.sql  commerce.product.00001.sql  commerce.product-schema.sql  commerce-schema-create.sql  metadata
```

### myloader

```
$ ./bin/myloader --help
Usage: ./bin/myloader -h [HOST] -P [PORT] -u [USER] -p [PASSWORD] -d  [DIR]
  -P int
    	TCP/IP port to connect to (default 3306)
  -d string
    	Directory of the dump to import
  -h string
    	The host to connect to
  -p string
    	User password
  -t int
    	Number of threads to use (default 16)
  -u string
    	Username with privileges to run the loader

Examples:
$./bin/myloader -h 192.168.0.2 -P 3306 -u mock -p mock -d sbtest.sql
 2017/10/25 13:04:17.396002 loader.go:75:         [INFO]        restoring.database[sbtest]
 2017/10/25 13:04:17.458076 loader.go:99:         [INFO]        restoring.schema[sbtest.benchyou0]
 2017/10/25 13:04:17.516236 loader.go:99:         [INFO]        restoring.schema[sbtest.benchyou1]
 2017/10/25 13:04:17.516389 loader.go:115:        [INFO]        restoring.tables[benchyou0].parts[00015].thread[1]
 2017/10/25 13:04:17.516456 loader.go:115:        [INFO]        restoring.tables[benchyou0].parts[00005].thread[2]

...
[stripped]
...

 2017/10/25 13:05:27.783560 loader.go:131:        [INFO]        restoring.tables[benchyou1].parts[00005].thread[9].done...
 2017/10/25 13:05:36.133758 loader.go:181:        [INFO]        restoring.allbytes[4087MB].time[78.62sec].rates[51.99MB/sec]...
 2017/10/25 13:05:44.759183 loader.go:131:        [INFO]        restoring.tables[benchyou0].parts[00001].thread[3].done...
 2017/10/25 13:05:46.133728 loader.go:181:        [INFO]        restoring.allbytes[4216MB].time[88.62sec].rates[47.58MB/sec]...
 2017/10/25 13:05:46.567156 loader.go:131:        [INFO]        restoring.tables[benchyou1].parts[00016].thread[6].done...
 2017/10/25 13:05:50.612200 loader.go:131:        [INFO]        restoring.tables[benchyou0].parts[00008].thread[10].done...
 2017/10/25 13:05:51.131155 loader.go:131:        [INFO]        restoring.tables[benchyou0].parts[00014].thread[2].done...
 2017/10/25 13:05:51.185629 loader.go:131:        [INFO]        restoring.tables[benchyou0].parts[00011].thread[1].done...
 2017/10/25 13:05:51.836354 loader.go:131:        [INFO]        restoring.tables[benchyou1].parts[00004].thread[0].done...
 2017/10/25 13:05:52.286931 loader.go:131:        [INFO]        restoring.tables[benchyou1].parts[00006].thread[11].done...
 2017/10/25 13:05:52.602444 loader.go:131:        [INFO]        restoring.tables[benchyou0].parts[00019].thread[8].done...
 2017/10/25 13:05:52.602573 loader.go:187:        [INFO]        restoring.all.done.cost[95.09sec].allbytes[5120.00MB].rate[53.85MB/s]
```

## License

go-mydumper is released under the GPLv3. See LICENSE
