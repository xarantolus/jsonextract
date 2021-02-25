cd internal/fuzz

# Generate fuzz data
echo "Generating fuzz data"
go test 

echo "Installing additional dependencies"
go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build

echo "Building fuzz package"
go-fuzz-build

echo "Fuzzing..."
go-fuzz
