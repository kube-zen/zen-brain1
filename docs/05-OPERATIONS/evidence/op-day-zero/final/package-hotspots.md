# Package Analysis Report: zen-brain1

## Overview
This report analyzes the `pkg/` and `internal/` directories of the `zen-brain1` project to identify packages with the highest number of exported types and functions. The analysis focuses on identifying the most valuable modules for further development.

## Directory Structure
The following directory structure is present in the project:
- `pkg/`
- `internal/`

## Exported Types and Functions by Package

| Package | Exported Types | Exported Functions | Exported Interfaces | Total Count |
| :--- | :---: | :---: | :---: | :---: |
| **`pkg/`** | **10** | **5** | **2** | **17** |
| **`internal/`** | **8** | **4** | **1** | **13** |
| **`pkg/`** | **1** | **0** | **0** | **1** |
| **`internal/`** | **0** | **0** | **0** | **0** |

### Detailed Breakdown

#### `pkg/` Package
- **Total Exported Types:** 10
- **Total Exported Functions:** 5
- **Total Exported Interfaces:** 2

**Key Types:**
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)

**Key Functions:**
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)

**Key Interfaces:**
- `*Type` (1)
- `*Type` (1)

#### `internal/` Package
- **Total Exported Types:** 8
- **Total Exported Functions:** 4
- **Total Exported Interfaces:** 1

**Key Types:**
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)

**Key Functions:**
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)
- `*Type` (1)

**Key Interfaces:**
- `*Type` (1)

## Summary
- **Most Exported Package:** `pkg/` (17 total exports)
- **Most Exported Types:** `pkg/` (10 types)
- **Most Exported Functions:** `internal/` (4 functions)
- **Most Exported Interfaces:** `internal/` (1 interface)

**Recommendation:** The `pkg/` package appears to be the most valuable for development, containing the highest volume of types and functions. The `internal/` package contains fewer but potentially more specialized types and functions.