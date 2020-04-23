
DB_NAME=test
PASSWORD=Passw0rd

help:
	echo "use start or stop target"

# Microsoft SQL Server

MSSQL_DB_FILES=/tmp/mssql_temp
MSSQL_CONTAINER_NAME=mssql_test
MSSQL_SA_PASSWORD=$(PASSWORD)

start-mssql:
	docker run --name=$(MSSQL_CONTAINER_NAME) -e 'ACCEPT_EULA=Y' -e 'MSSQL_SA_PASSWORD=$(MSSQL_SA_PASSWORD)' -e 'MSSQL_PID=Developer' --cap-add SYS_PTRACE -v $(MSSQL_DB_FILES):/var/opt/mssql -d -p 1433:1433 microsoft/mssql-server-linux
	echo -n "starting $(MSSQL_CONTAINER_NAME) "; while ! docker logs $(MSSQL_CONTAINER_NAME) 2>&1 | grep SQL.Server.is.now.ready.for.client.connections >/dev/null ; do echo -n .; sleep 1; done; echo " done"
	docker exec $(MSSQL_CONTAINER_NAME) /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P '$(MSSQL_SA_PASSWORD)' -Q 'create database $(DB_NAME)'

test-mssql:
	go test -v -mssrv=localhost -msdb=$(DB_NAME) -msuser=sa -mspass=$(MSSQL_SA_PASSWORD) -run=TestMSSQL

test-mssql-race:
	go test -v -mssrv=localhost -msdb=$(DB_NAME) -msuser=sa -mspass=$(MSSQL_SA_PASSWORD) -run=TestMSSQL --race

stop-mssql:
	docker stop $(MSSQL_CONTAINER_NAME)
	docker rm $(MSSQL_CONTAINER_NAME)

# MySQL

MYSQL_CONTAINER_NAME=mysql_test
MYSQL_ROOT_PASSWORD=$(PASSWORD)

start-mysql:
	docker run --name=$(MYSQL_CONTAINER_NAME) -e 'MYSQL_ROOT_PASSWORD=$(MYSQL_ROOT_PASSWORD)' -d -p 127.0.0.1:3306:3306 mysql
	echo -n "starting $(MYSQL_CONTAINER_NAME) "; while ! docker logs $(MYSQL_CONTAINER_NAME) 2>&1 | grep ^Version.*port:.3306 >/dev/null ; do echo -n .; sleep 1; done; echo " done"
	docker exec $(MYSQL_CONTAINER_NAME) sh -c 'echo "create database $(DB_NAME)" | MYSQL_PWD=$(MYSQL_ROOT_PASSWORD) mysql -hlocalhost -uroot'

test-mysql:
	go test -v -mydb=$(DB_NAME) -mypass=$(MYSQL_ROOT_PASSWORD) -mysrv=127.0.0.1 -myuser=root -run=MYSQL

stop-mysql:
	docker stop $(MYSQL_CONTAINER_NAME)
	docker rm $(MYSQL_CONTAINER_NAME)
