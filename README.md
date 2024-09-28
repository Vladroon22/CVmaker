# CVmaker
<h1> The microservice CV maker <h1>

<h2>Configuration</h2>

```
sudo docker run --name=CV -e POSTGRES_PASSWORD=55555 -p 5433:5432 -d postgres:16.2
```

<h3>Export env variables</h3>

```
export DB="postgres:55555@localhost:5433/postgres?sslmode=disable" 
export JWT="imagine your own secret key"

```

<h2>How to run</h2>

``` make run ```

