#!/bin/sh
#set -e

MSSQL_DRIVER_NAME="$1"
MSSQL_CONTAINER_NAME="$2"
DB_NAME="$3"
MSSQL_SA_PASSWORD="$4"
RACE="$5"

pwd

echo $PATH

go version

#sqlcmd -S localhost
#sqlcmd -?

# install freetds driver
cat << EOF > tds.driver.template
[FreeTDS]
Driver = /usr/lib/x86_64-linux-gnu/odbc/libtdsodbc.so
EOF
odbcinst -i -d -f tds.driver.template
cat tds.driver.template
rm tds.driver.template

# install microsoft driver
odbcinst -q -d -n "${MSSQL_DRIVER_NAME}" > tds.driver.template
odbcinst -i -d -f tds.driver.template
cat tds.driver.template
rm tds.driver.template

echo "MSSQL_DRIVER_NAME=${MSSQL_DRIVER_NAME}"
echo "MSSQL_CONTAINER_NAME=${MSSQL_CONTAINER_NAME}"
echo "DB_NAME=${DB_NAME}"
echo "MSSQL_SA_PASSWORD=${MSSQL_SA_PASSWORD}"
echo "RACE=${RACE}"

# add 1433 to mssrv parameter so we do not skip TestMSSQLReconnect

go test -v \
	-msdriver="${MSSQL_DRIVER_NAME}" \
	-mssrv=${MSSQL_CONTAINER_NAME},1433 \
	-msdb=${DB_NAME} \
	-msuser=sa \
	-mspass=${MSSQL_SA_PASSWORD} \
	${RACE} \
	-run=TestMSSQL
