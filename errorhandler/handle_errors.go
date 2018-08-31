package errorhandler

func ErrorHandler(errorChans ...<-chan error) <-chan error {
	collector := make(chan error)

	for _, errChan := range errorChans {
		go func(c <-chan error) {
			for e := range c {
				collector <- e
			}
		}(errChan)
	}

	return collector
}
