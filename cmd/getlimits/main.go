package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Printf("INT_MIN=%d\n", math.MinInt32)
	fmt.Printf("INT_MAX=%d\n", math.MaxInt32)
	fmt.Printf("INT_OFLOW=%d\n", int64(math.MaxInt32)+1)
	fmt.Printf("INT_UFLOW=%d\n", int64(math.MinInt32)-1)
	fmt.Printf("UINT_MAX=%d\n", math.MaxUint32)
	fmt.Printf("UINT_OFLOW=%d\n", int64(math.MaxUint32)+1)
	fmt.Printf("LONG_MIN=%d\n", math.MinInt64)
	fmt.Printf("LONG_MAX=%d\n", math.MaxInt64)
	fmt.Printf("ULONG_MAX=%d\n", uint64(math.MaxUint64))
	fmt.Printf("ULONG_OFLOW=18446744073709551616\n")
	fmt.Printf("SIZE_MAX=%d\n", uint64(math.MaxUint64))
	fmt.Printf("SIZE_OFLOW=18446744073709551616\n")
	fmt.Printf("SSIZE_MAX=%d\n", math.MaxInt64)
	fmt.Printf("SSIZE_OFLOW=%d\n", uint64(math.MaxInt64)+1)
	fmt.Printf("INTMAX_MAX=%d\n", math.MaxInt64)
	fmt.Printf("INTMAX_MIN=%d\n", math.MinInt64)
	fmt.Printf("INTMAX_OFLOW=%d\n", uint64(math.MaxInt64)+1)
	fmt.Printf("INTMAX_UFLOW=-9223372036854775809\n")
	fmt.Printf("UINTMAX_MAX=%d\n", uint64(math.MaxUint64))
	fmt.Printf("UINTMAX_OFLOW=18446744073709551616\n")
	fmt.Printf("OFF_T_MAX=%d\n", math.MaxInt64)
	fmt.Printf("OFF_T_MIN=%d\n", math.MinInt64)
	fmt.Printf("OFF_T_OFLOW=%d\n", uint64(math.MaxInt64)+1)
	fmt.Printf("OFF_T_UFLOW=%d\n", math.MinInt64)
	fmt.Printf("UID_MAX=%d\n", math.MaxUint32)
	fmt.Printf("GID_MAX=%d\n", math.MaxUint32)
	fmt.Printf("PID_T_MAX=%d\n", math.MaxInt32)
}
