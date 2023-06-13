package services

type ICollector interface {
	Collect(data string) error
}

var localCollector ICollector

func Collector() ICollector {
	if localCollector == nil {
		panic("impl not found for ICollector")
	}
	return localCollector
}

func RegisterCollector(i ICollector) {
	localCollector = i
}
