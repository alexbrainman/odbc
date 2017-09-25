
DB_FILES=/tmp/mssql_temp
CONTAINER_NAME=mssql_test
SA_PASSWORD=Passw0rd
DB_NAME=test

help:
	echo "use start or stop target"

start:
	docker run --name=$(CONTAINER_NAME) -e 'ACCEPT_EULA=Y' -e 'MSSQL_SA_PASSWORD=$(SA_PASSWORD)' -e 'MSSQL_PID=Developer' --cap-add SYS_PTRACE -v $(DB_FILES):/var/opt/mssql -d -p 1433:1433 microsoft/mssql-server-linux
	sleep 10
	docker exec mssql_test /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P '$(SA_PASSWORD)' -Q 'create database $(DB_NAME)'

stop:
	docker stop $(CONTAINER_NAME)
	docker rm $(CONTAINER_NAME)
