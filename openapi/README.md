## Show API specs in Swagger Editor

To successfully show Open API specs from several files in Swagger Editor, you have to: 

1. Run Swagger using `docker-compose.yaml`:
   ```bash
   docker-compose up
   ```

2. Go to Swagger edit URL:
   ```
   http://localhost:8081/?url=/docs/internal_api.yaml
   ```

For reference, see the [issue](https://github.com/swagger-api/swagger-editor/issues/1409) related to Swagger not being able to show specs from several files.
