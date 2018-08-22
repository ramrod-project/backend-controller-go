#!/bin/bash

./cc-test-reporter before-build 
for pkg in $(go list ./... | grep -v main); do
    if [[ $pkg == "github.com/ramrod-project/backend-controller-go/test" ]]; then
        continue
    fi
    go test -v -parallel 1 -coverprofile=$(echo $pkg | tr / -).cover $pkg
    if ! [[ $? == 0 ]]; then
        exit 1
    fi
done
echo "mode: set" > c.out
grep -h -v "^mode:" ./*.cover >> c.out
rm -f *.cover

./cc-test-reporter after-build