# The microservice CV maker 

<h2>Configuration</h2>

```
sudo docker run --name=CV -e POSTGRES_PASSWORD=55555 -p 5433:5432 -d postgres:16.2
sudo docker run --name=MyRedis -p 6379:6379 -d redis
```

<h3>Export env variables</h3>

```
export DB="postgres:55555@localhost:5433/postgres?sslmode=disable" 
export KEY="imagine your own secret key"
```

<h2>How to run</h2>

``` make run ```

