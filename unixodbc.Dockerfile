FROM ubuntu:18.04

RUN apt update -y  &&  apt upgrade -y && apt-get update
RUN apt install -y curl python3.7 git python3-pip openjdk-8-jdk unixodbc-dev

# Add SQL Server ODBC Driver 17 for Ubuntu 18.04
RUN curl https://packages.microsoft.com/keys/microsoft.asc | apt-key add -
RUN curl https://packages.microsoft.com/config/ubuntu/18.04/prod.list > /etc/apt/sources.list.d/mssql-release.list
RUN apt-get update
RUN ACCEPT_EULA=Y apt-get install -y --allow-unauthenticated msodbcsql17
RUN ACCEPT_EULA=Y apt-get install -y --allow-unauthenticated mssql-tools
RUN echo 'export PATH="$PATH:/opt/mssql-tools/bin"' >> ~/.bash_profile
RUN echo 'export PATH="$PATH:/opt/mssql-tools/bin"' >> ~/.bashrc
ENV PATH ${PATH}:/opt/mssql-tools/bin

# download freetds driver
RUN apt-get install -y unixodbc freetds-dev freetds-bin tdsodbc

# download go tar
RUN curl --location https://go.dev/dl/go1.20.linux-amd64.tar.gz | tar -C /usr/local -xzf -
RUN echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.bash_profile
RUN echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.bashrc
ENV PATH /usr/local/go/bin:${PATH}

WORKDIR /src

#COPY mssqltest.sh /
#RUN chmod +x /mssqltest.sh
#ENTRYPOINT ["sh","/mssqltest.sh"]
#ENTRYPOINT ["sh"]
CMD ["sh"]
