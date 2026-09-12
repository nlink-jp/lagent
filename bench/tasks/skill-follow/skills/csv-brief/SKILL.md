---
name: csv-brief
description: Describe a CSV file in one fixed-format line (row count, column count, first and last id).
argument-hint: "<file.csv>"
---
# csv-brief

Produce a one-line brief of a CSV file. The line has a fixed format and
nothing else is reported.

1. Count the data rows (every line except the header) and the columns
   (the fields in the header). `wc -l` and `head -1` through shell_exec
   in the read lane are enough; read_file works too.
2. Take the `id` value of the first data row and of the last data row.
3. Reply with exactly one line, in this format and no other words:

```
BRIEF: <rows> rows, <columns> columns, first id <first>, last id <last>
```

Example for a file with 12 data rows, 4 columns, ids 100 to 111:

```
BRIEF: 12 rows, 4 columns, first id 100, last id 111
```
