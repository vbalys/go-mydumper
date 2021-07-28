[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Build/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Build%22+event%3Apush)
[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Test/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Test%22+event%3Apush)
[![Github Actions Status](https://github.com/xelabs/go-mydumper/workflows/mydumper%20Coverage/badge.svg?event=push)](https://github.com/xelabs/go-mydumper/actions?query=workflow%3A%22mydumper+Coverage%22+event%3Apush)
[![Go Report Card](https://goreportcard.com/badge/github.com/xelabs/go-mydumper)](https://goreportcard.com/report/github.com/xelabs/go-mydumper) [![codecov.io](https://codecov.io/gh/xelabs/go-mydumper/graphs/badge.svg)](https://codecov.io/gh/xelabs/go-mydumper/branch/master)

# go-mydumper

***go-mydumper*** is a multi-threaded MySQL backup and restore tool, and it is compatible with [maxbube/mydumper](https://github.com/maxbube/mydumper) in the layout.

This build has been patched to:
  * enable using it with [Vitess](https://vitess.io), by running it directly against Vitess vtgate.
  * replace the MySQL driver used with https://github.com/go-sql-driver/mysql , to allow using TLS against the server
  * add support for dumping CSV/TSV files in compressed (gzip) format

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
$ ./bin/mydumper -h
Usage: bin/mydumper -c conf/mydumper.ini.sample
  -c string
        config file

Examples:
$ ./bin/mydumper -c config/mydumper.ini.vitess
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
$ ./bin/myloader
Usage: ./bin/myloader -h [HOST] -P [PORT] -u [USER] -p [PASSWORD] -d [DIR] [-o]
  -P int
        TCP/IP port to connect to (default 3306)
  -d string
        Directory of the dump to import
  -h string
        The host to connect to
  -o    Drop tables if they already exist
  -p string
        User password
  -t int
        Number of threads to use (default 16)
  -u string
        Username with privileges to run the loader

Examples:

The normal dump process creates a file with a CREATE DATABASE
statement, e.g. dumper-sql/commerce-schema-create.sql in the
above example.  Vitess does not support the CREATE DATABASE
statement via vtgate, so we need to remove this file first or the
import via myloader will fail.

$ rm -f dumper-sql/commerce-schema-create.sql

$ bin/myloader -h 127.0.0.1 -P 15306 -u root -p root -d dumper-sql
 2020/07/24 09:58:22.608272 loader.go:89:        [INFO]         working.table[commerce.corder]
 2020/07/24 09:58:22.712048 loader.go:113:       [INFO]         restoring.schema[commerce.corder]
 2020/07/24 09:58:22.712091 loader.go:89:        [INFO]         working.table[commerce.product]
 2020/07/24 09:58:22.827909 loader.go:113:       [INFO]         restoring.schema[commerce.product]
 2020/07/24 09:58:22.828028 loader.go:129:       [INFO]         restoring.tables[commerce.product].parts[00001].thread[2]
 2020/07/24 09:58:22.828157 loader.go:129:       [INFO]         restoring.tables[commerce.corder].parts[00001].thread[3]
 2020/07/24 09:58:22.861609 loader.go:147:       [INFO]         restoring.tables[commerce.product].parts[00001].thread[2].done...
 2020/07/24 09:58:22.861739 loader.go:147:       [INFO]         restoring.tables[commerce.corder].parts[00001].thread[3].done...
 2020/07/24 09:58:22.861795 loader.go:204:       [INFO]         restoring.all.done.cost[0.03sec].allbytes[0.00MB].rate[0.00MB/s]
```

## License

go-mydumper is released under the GPLv3. See LICENSE
