# The microservice CV maker 

<h2>Configuration</h2>

```
sudo docker run --name=CV -e POSTGRES_PASSWORD=55555 -p 5433:5432 -d postgres:16.2
sudo docker run --name=MyRedis -p 6379:6379 -d redis
```

<h3>Export env variables in .env file</h3>

```
addr="localhost"
port="8080"
portS="8443"
DB="postgres://postgres:55555@locahost:5433/postgres?sslmode=disable"
Redis="6379"
KEY="imagine your secret key"
cert="cert.crt"
keys="Key.key"
```

<h2>How to run</h2>

<h4>For SSL/TLS here used self-signed certificates<h4>

<h5>HTTPS way

```
make ssl  
```

then 

```
make run 
```

<h5>HTTP way </h5>

```
make run 
```

<h5>Containerization way</h4>

```
make compose-run
```
