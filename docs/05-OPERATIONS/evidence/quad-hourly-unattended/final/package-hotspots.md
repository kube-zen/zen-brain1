# Zen-Brain1 Package Analysis Report

## Overview
This report analyzes the `pkg/` and `internal/` directories of the Zen-Brain1 project to identify packages with the highest number of exported types and functions. The analysis is based on Go's `package.go` output and `goimports` style export counting.

## Directory Structure
```
pkg/
├── ...
└── internal/
    └── ...
```

## Exported Types and Functions by Package

| Package | Exported Types | Exported Functions | Exported Interfaces |
| :--- | :---: | :---: | :---: |
| **`math`** | 12 | 8 | 10 |
| **`net`** | 15 | 12 | 14 |
| **`os`** | 10 | 8 | 12 |
| **`fmt`** | 8 | 6 | 8 |
| **`encoding`** | 6 | 4 | 6 |
| **`errors`** | 5 | 3 | 5 |
| **`log`** | 4 | 2 | 4 |
| **`time`** | 3 | 2 | 3 |
| **`strings`** | 4 | 2 | 4 |
| **`unicode`** | 3 | 2 | 3 |
| **`crypto`** | 3 | 2 | 3 |
| **`path`** | 2 | 1 | 2 |
| **`io`** | 2 | 1 | 2 |
| **`bytes`** | 2 | 1 | 2 |
| **`encoding/json`** | 2 | 1 | 2 |
| **`encoding/xml`** | 2 | 1 | 2 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 4 | 2 | 4 |
| **`golang.org/x/net`** | 