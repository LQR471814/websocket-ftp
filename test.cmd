cd client
call npm run build
cd ..\test_client
call npm run build

cd ..\server
go test -coverprofile="coverage.out"
go tool cover -html=coverage.out -o coverage.html
start coverage.html