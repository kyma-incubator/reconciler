## Swagger Editor

To be able to show Open API specs from several files without error in Swagger Editor, you have to: 
1. Run swagger using `docker-compose.yaml`.
   ```bash
   docker-compose up
   ```

2. Go to Swagger edit URL:`http://localhost:8081/?url=/docs/internal_api.yaml`


Here is the [issue](https://github.com/swagger-api/swagger-editor/issues/1409) about error.
