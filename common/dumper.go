/*
 * go-mydumper
 * xelabs.org
 *
 * Copyright (c) XeLabs
 * GPL License
 *
 */

package common

import (
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xelabs/go-mydumper/config"
	"github.com/xelabs/go-mysqlstack/xlog"
)

func writeMetaData(args *config.Config) {
	file := fmt.Sprintf("%s/metadata", args.Outdir)
	WriteFile(file, "")
}

func dumpDatabaseSchema(log *xlog.Log, conn *Connection, args *config.Config, database string) {
	schema := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", database)
	file := fmt.Sprintf("%s/%s-schema-create.sql", args.Outdir, database)
	WriteFile(file, schema)
	log.Info("dumping.database[%s].schema...", database)
}

func dumpTableSchema(log *xlog.Log, conn *Connection, args *config.Config, database string, table string) {
	rows, err := conn.StreamFetch(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", database, table))
	AssertNil(err)
	rows.Next()

	var tableName, createTable string
	err = rows.Scan(&tableName, &createTable)
	AssertNil(err)
	rows.Close()

	schema := createTable + ";\n"

	file := fmt.Sprintf("%s/%s.%s-schema.sql", args.Outdir, database, table)
	WriteFile(file, schema)
	log.Info("dumping.table[%s.%s].schema...", database, table)
}

// Dump a table in "MySQL" (multi-inserts) format
func dumpTable(log *xlog.Log, conn *Connection, args *config.Config, database string, table string) {
	var allBytes uint64
	var allRows uint64
	var where string
	var selfields []string

	fields := make([]string, 0, 16)
	fieldTypes := make([]string, 0, 16)
	{
		rows, err := conn.StreamFetch(fmt.Sprintf("SELECT * FROM `%s`.`%s` LIMIT 1", database, table))
		AssertNil(err)

		colTypes, err := rows.ColumnTypes()
		AssertNil(err)

		for _, colType := range colTypes {
			log.Debug("dump -- %#v, %s, %s", args.Filters, table, colType.Name())

			if _, ok := args.Filters[table][colType.Name()]; ok {
				continue
			}

			fields = append(fields, fmt.Sprintf("`%s`", colType.Name()))
			fieldTypes = append(fieldTypes, colType.DatabaseTypeName())
			replacement, ok := args.Selects[table][colType.Name()]
			if ok {
				selfields = append(selfields, fmt.Sprintf("%s AS `%s`", replacement, colType.Name()))
			} else {
				selfields = append(selfields, fmt.Sprintf("`%s`", colType.Name()))
			}
		}
		err = rows.Close()
		AssertNil(err)
	}

	if v, ok := args.Wheres[table]; ok {
		where = fmt.Sprintf(" WHERE %v", v)
	}

	rows, err := conn.StreamFetch(fmt.Sprintf("SELECT %s FROM `%s`.`%s` %s", strings.Join(selfields, ", "), database, table, where))
	AssertNil(err)

	fileNo := 1
	stmtsize := 0

	colNames, err := rows.Columns()
	AssertNil(err)
	cols := make([]interface{}, len(colNames))
	colPtrs := make([]interface{}, len(colNames))
	for i := 0; i < len(colNames); i++ {
		colPtrs[i] = &cols[i]
	}

	chunkbytes := 0
	outputRows := make([]string, 0, 256)
	inserts := make([]string, 0, 256)
	for rows.Next() {
		err := rows.Scan(colPtrs...)
		AssertNil(err)

		values := make([]string, 0, 16)
		rowsize := 3  // start and end paren plus newline
		for i, col := range cols {
			colKind := fieldTypes[i]
			if col == nil || colKind == "NULL" {
				values = append(values, "NULL")
				rowsize += 4
			} else {
				switch colKind {
				case "BIGINT", "INT", "SMALLINT", "MEDIUMINT", "TINYINT", "DECIMAL", "FLOAT":
					str := fmt.Sprintf("%s", col)
					values = append(values, str)
					rowsize += len(str)
				case "CHAR", "VARCHAR", "TEXT", "LONGTEXT", "MEDIUMTEXT", "TINYTEXT":
					byteCol := col.([]byte)
					values = append(values, fmt.Sprintf(`"%s"`, EscapeBytes(byteCol)))
					rowsize += len(byteCol) + 2
				default:
					// TODO:  check/test the following types
					// "BINARY" "BIT" "BLOB" "DATE" "DATETIME" "DOUBLE" "ENUM" "GEOMETRY"
					// "JSON" "LONGBLOB" "MEDIUMBLOB" "SET" "TIME" "TIMESTAMP" "TINYBLOB"
					// "VARBINARY" "YEAR"
					byteCol := col.([]byte)
					values = append(values, fmt.Sprintf(`"%s"`, EscapeBytes(byteCol)))
					rowsize += len(byteCol) + 2
				}
			}
		}
		r := "(" + strings.Join(values, ",") + ")"
		outputRows = append(outputRows, r)

		allRows++
		stmtsize += len(r)
		chunkbytes += len(r)
		allBytes += uint64(len(r))
		atomic.AddUint64(&args.Allbytes, uint64(len(r)))
		atomic.AddUint64(&args.Allrows, 1)

		if stmtsize >= args.StmtSize {
			insertone := fmt.Sprintf("INSERT INTO `%s`(%s) VALUES\n%s", table, strings.Join(fields, ","), strings.Join(outputRows, ",\n"))
			inserts = append(inserts, insertone)
			outputRows = outputRows[:0]
			stmtsize = 0
		}

		if (chunkbytes / 1024 / 1024) >= args.ChunksizeInMB {
			query := strings.Join(inserts, ";\n") + ";\n"
			file := fmt.Sprintf("%s/%s.%s.%05d.sql", args.Outdir, database, table, fileNo)
			WriteFile(file, query)

			log.Info("dumping.table[%s.%s].rows[%v].bytes[%vMB].part[%v].thread[%d]", database, table, allRows, allBytes / 1024 / 1024, fileNo, conn.ID)
			inserts = inserts[:0]
			chunkbytes = 0
			fileNo++
		}
	}
	if chunkbytes > 0 {
		if len(outputRows) > 0 {
			insertone := fmt.Sprintf("INSERT INTO `%s`(%s) VALUES\n%s", table, strings.Join(fields, ","), strings.Join(outputRows, ",\n"))
			inserts = append(inserts, insertone)
		}

		query := strings.Join(inserts, ";\n") + ";\n"
		file := fmt.Sprintf("%s/%s.%s.%05d.sql", args.Outdir, database, table, fileNo)
		WriteFile(file, query)
	}
	err = rows.Close()
	AssertNil(err)

	log.Info("dumping.table[%s.%s].done.allrows[%v].allbytes[%vMB].thread[%d]...", database, table, allRows, allBytes / 1024 / 1024, conn.ID)
}

// Dump a table in CSV/TSV format
func dumpTableCsv(log *xlog.Log, conn *Connection, args *config.Config, database string, table string, separator rune, compress bool) {
	var allBytes uint64
	var allRows uint64
	var where string
	var selfields []string
	var headerfields []string

	fields := make([]string, 0, 16)
	fieldTypes := make([]string, 0, 16)
	{
		rows, err := conn.StreamFetch(fmt.Sprintf("SELECT * FROM `%s`.`%s` LIMIT 1", database, table))
		AssertNil(err)

		colTypes, err := rows.ColumnTypes()
		AssertNil(err)

		for _, colType := range colTypes {
			log.Debug("dump -- %#v, %s, %s", args.Filters, table, colType.Name())

			if _, ok := args.Filters[table][colType.Name()]; ok {
				continue
			}

			fields = append(fields, fmt.Sprintf("`%s`", colType.Name()))
			fieldTypes = append(fieldTypes, colType.DatabaseTypeName())
			headerfields = append(headerfields, colType.Name())
			replacement, ok := args.Selects[table][colType.Name()]
			if ok {
				selfields = append(selfields, fmt.Sprintf("%s AS `%s`", replacement, colType.Name()))
			} else {
				selfields = append(selfields, fmt.Sprintf("`%s`", colType.Name()))
			}
		}
		err = rows.Close()
		AssertNil(err)
	}

	if v, ok := args.Wheres[table]; ok {
		where = fmt.Sprintf(" WHERE %v", v)
	}

	rows, err := conn.StreamFetch(fmt.Sprintf("SELECT %s FROM `%s`.`%s` %s", strings.Join(selfields, ", "), database, table, where))
	AssertNil(err)

	fileNo := 1
	extension := "csv"
	if compress {
		extension = extension + ".gz"
	}
	file, err := os.Create(fmt.Sprintf("%s/%s.%s.%05d.%s", args.Outdir, database, table, fileNo, extension))
	AssertNil(err)

	var writer *csv.Writer
	var zipWriter *gzip.Writer
	if compress {
		zipWriter, _ = gzip.NewWriterLevel(file, 1)
		writer = csv.NewWriter(zipWriter)
	} else {
		writer = csv.NewWriter(file)
	}
	writer.Comma = separator
	writer.Write(headerfields)

	colNames, err := rows.Columns()
	AssertNil(err)
	cols := make([]interface{}, len(colNames))
	colPtrs := make([]interface{}, len(colNames))
	for i := 0; i < len(colNames); i++ {
		colPtrs[i] = &cols[i]
	}

	chunkbytes := 0
	outputRows := make([]string, 0, 256)
	outputRows = append(outputRows, strings.Join(headerfields, "\t"))
	inserts := make([]string, 0, 256)
	for rows.Next() {
		err := rows.Scan(colPtrs...)
		AssertNil(err)

		values := make([]string, 0, 16)
		rowsize := 3  // start and end paren plus newline
		for i, col := range cols {
			colKind := fieldTypes[i]
			if col == nil || colKind == "NULL" {
				values = append(values, "NULL")
				rowsize += 4
			} else {
				switch colKind {
				case "BIGINT", "INT", "SMALLINT", "MEDIUMINT", "TINYINT", "DECIMAL", "FLOAT":
					str := fmt.Sprintf("%s", col)
					values = append(values, str)
					rowsize += len(str)
				case "CHAR", "VARCHAR", "TEXT", "LONGTEXT", "MEDIUMTEXT", "TINYTEXT":
					str := fmt.Sprintf(`%s`, col)
					values = append(values, str)
					rowsize += len(str)
				default:
					// TODO:  check/test the following types
					// "BINARY" "BIT" "BLOB" "DATE" "DATETIME" "DOUBLE" "ENUM" "GEOMETRY"
					// "JSON" "LONGBLOB" "MEDIUMBLOB" "SET" "TIME" "TIMESTAMP" "TINYBLOB"
					// "VARBINARY" "YEAR"
					byteCol := col.([]byte)
					values = append(values, fmt.Sprintf("%s", EscapeBytes(byteCol)))
					rowsize += len(byteCol) + 2
				}
			}
		}
		writer.Write(values)
		chunkbytes += rowsize

		allRows++
		atomic.AddUint64(&args.Allbytes, uint64(rowsize))
		atomic.AddUint64(&args.Allrows, 1)

		if (chunkbytes / 1024 / 1024) >= args.ChunksizeInMB {
			log.Info("dumping.table[%s.%s].rows[%v].bytes[%vMB].part[%v].thread[%d]", database, table, allRows, allBytes / 1024 / 1024, fileNo, conn.ID)
			writer.Flush()
			if zipWriter != nil {
				zipWriter.Close()
			}
			file.Close()
			fileNo++
			file, err := os.Create(fmt.Sprintf("%s/%s.%s.%05d.%s", args.Outdir, database, table, fileNo, extension))
			AssertNil(err)
			if compress {
				zipWriter, _ = gzip.NewWriterLevel(file, 1)
				writer = csv.NewWriter(zipWriter)
			} else {
				writer = csv.NewWriter(file)
			}
			writer.Comma = separator
			writer.Write(headerfields)
			log.Info("dumping.table[%s.%s].rows[%v].bytes[%vMB].part[%v].thread[%d]", database, table, allRows, allBytes / 1024 / 1024, fileNo, conn.ID)
			inserts = inserts[:0]
			chunkbytes = 0
		}
	}

	writer.Flush()
	if zipWriter != nil {
		zipWriter.Close()
	}
	file.Close()
	err = rows.Close()
	AssertNil(err)

	log.Info("dumping.table[%s.%s].done.allrows[%v].allbytes[%vMB].thread[%d]...", database, table, allRows, allBytes / 1024 / 1024, conn.ID)
}

func allTables(log *xlog.Log, conn *Connection, database string) []string {
	rows, err := conn.StreamFetch(fmt.Sprintf("SHOW TABLES FROM `%s`", database))
	AssertNil(err)

	tables := make([]string, 0, 128)
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		tables = append(tables, tableName)
	}
	fmt.Printf("tables: %v\n", tables)
	return tables
}

func allDatabases(log *xlog.Log, conn *Connection) []string {
	rows, err := conn.StreamFetch("SHOW DATABASES")
	AssertNil(err)

	databases := make([]string, 0, 128)
	for rows.Next() {
		var databaseName string
		err = rows.Scan(&databaseName)
		databases = append(databases, databaseName)
	}
	rows.Close()
	return databases
}

func filterDatabases(log *xlog.Log, conn *Connection, filter *regexp.Regexp, invert bool) []string {
	rows, err := conn.StreamFetch("SHOW DATABASES")
	AssertNil(err)

	databases := make([]string, 0, 128)
	for rows.Next() {
		var databaseName string
		err = rows.Scan(&databaseName)
		if (!invert && filter.MatchString(databaseName)) || (invert && !filter.MatchString(databaseName)) {
			databases = append(databases, databaseName)
		}
	}
	rows.Close()
	return databases
}

// Dumper used to start the dumper worker.
func Dumper(log *xlog.Log, args *config.Config) {
	initPool, err := NewPool(log, args.Threads, args.Address, args.User, args.Password, args.InitVars, "")
	AssertNil(err)
	defer initPool.Close()

	// Meta data.
	writeMetaData(args)

	// database.
	conn := initPool.Get()
	var databases []string
	t := time.Now()
	if args.DatabaseRegexp != "" {
		r := regexp.MustCompile(args.DatabaseRegexp)
		databases = filterDatabases(log, conn, r, args.DatabaseInvertRegexp)
	} else {
		if args.Database != "" {
			databases = strings.Split(args.Database, ",")
		} else {
			databases = allDatabases(log, conn)
		}
	}
	for _, database := range databases {
		dumpDatabaseSchema(log, conn, args, database)
	}

	tables := make([][]string, len(databases))
	for i, database := range databases {
		if args.Table != "" {
			tables[i] = strings.Split(args.Table, ",")
		} else {
			tables[i] = allTables(log, conn, database)
		}
	}
	initPool.Put(conn)

	var wg sync.WaitGroup
	for i, database := range databases {
		pool, err := NewPool(log, args.Threads/len(databases), args.Address, args.User, args.Password, args.SessionVars, database)
		AssertNil(err)
		defer pool.Close()
		for _, table := range tables[i] {
			conn := initPool.Get()
			dumpTableSchema(log, conn, args, database, table)
			initPool.Put(conn)

			conn = pool.Get()
			wg.Add(1)
			go func(conn *Connection, database string, table string) {
				defer func() {
					wg.Done()
					pool.Put(conn)
				}()
				log.Info("dumping.table[%s.%s].datas.thread[%d]...", database, table, conn.ID)
				if args.Format == "mysql" || args.Format == "" {
					dumpTable(log, conn, args, database, table)
				} else if args.Format == "tsv" {
					dumpTableCsv(log, conn, args, database, table, '\t', false)
				} else if args.Format == "tsv.gz" {
					dumpTableCsv(log, conn, args, database, table, '\t', true)
				} else if args.Format == "csv" {
					dumpTableCsv(log, conn, args, database, table, ',', false)
				} else if args.Format == "csv.gz" {
					dumpTableCsv(log, conn, args, database, table, ',', true)
				} else {
					AssertNil(errors.New(fmt.Sprintf("unknown dump format: [%v]", args.Format)))
				}

				log.Info("dumping.table[%s.%s].datas.thread[%d].done...", database, table, conn.ID)
			}(conn, database, table)
		}
	}

	tick := time.NewTicker(time.Millisecond * time.Duration(args.IntervalMs))
	defer tick.Stop()
	go func() {
		for range tick.C {
			diff := time.Since(t).Seconds()
			allbytesMB := float64(atomic.LoadUint64(&args.Allbytes) / 1024 / 1024)
			allrows := atomic.LoadUint64(&args.Allrows)
			rates := allbytesMB / diff
			log.Info("dumping.allbytes[%vMB].allrows[%v].time[%.2fsec].rates[%.2fMB/sec]...", allbytesMB, allrows, diff, rates)
		}
	}()

	wg.Wait()
	elapsed := time.Since(t).Seconds()
	log.Info("dumping.all.done.cost[%.2fsec].allrows[%v].allbytes[%v].rate[%.2fMB/s]", elapsed, args.Allrows, args.Allbytes, float64(args.Allbytes/1024/1024) / elapsed)
}
