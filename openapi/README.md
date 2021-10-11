# Prerequisites

- [oapi-codegen](https://github.com/deepmap/oapi-codegen)
- [spectral](https://github.com/stoplightio/spectral)

## Generation and validation

To generate types go to the repo root directory and run the following command:

```bash
   make generate-oapi-models
```

To validate the correctness of the manifest go to the repo root directory and run the following command:

```bash
   make validate-oapi-spec
```

## Show API specs in Swagger Editor

To successfully show Open API specs from several files in Swagger Editor, you have to:

1. Run Swagger using `docker-compose.yaml`:

   ```bash
   docker-compose up
   ```

2. Go to Swagger edit URL:

   ```text
   http://localhost:8081/?url=/docs/internal_api.yaml
   ```

For reference, see the [issue](https://github.com/swagger-api/swagger-editor/issues/1409) related to Swagger not being able to show specs from several files.
