package main

import "time"

// statementTimeout bounds every API query (DATABASE.md §10).
const statementTimeout = 5 * time.Second
