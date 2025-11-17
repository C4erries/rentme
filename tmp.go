package main

func wrap[T any](x any) T {
	return x.(T)
}

func main() {}
