# Configuration Management

## Requirements

* **Buckets**
    * Configuration values are grouped into buckets (e.g. default bucket, landscape- or customer-specific buckets etc.).
    * Configuration entries with the same key can exists in multiple buckets.
    * It must be possible to merge multiple buckets (overlay of buckets): Configuration entries of a lower-layered
      bucket are overwritten by an entry with the same key of a higher-layered bucket.
    * The number and the ordering of merged buckets can be defined flexibly.
* **Configuration entries**
    * A configuration entry is always a combination of a key and a value entity (like `my.config.key=abc`).
    * For auditing purposes, it must always be possible to identify who created or modified a key, and when it happened.
    * Configuration key entity:
        * A configuration key entity contains, beside its unique key (e.g. `my.config.key`), further metadata:
            * Creation date of the key entry
            * User who created the it
            * Data type of the value (e.g. String, Integer, Boolean)
            * Validation logic to verify the value (e.g. checking min-max constraints)
            * **TBC:** One or more (pre-defined) trigger function, which is executed by the reconciler. It can be
              specified whether the trigger should run at the beginning or at the end of a reconciliation cycle.
        * Configuration key entities are immutable and versioned: Changing any metadata leads to a new version of the
          configuration key entity.
    * Configuration value entity:
        * A configuration value entity is a mapping between the value (e.g. `abc`) and a configuration key entry.
        * A configuration value entry also contains metadata:
            * Name of the bucket the value belongs to
            * Creation date of the entry
            * User who created the configuration value entry
        * When a configuration value entity is created, by default, it is mapped to the latest version of a
          configuration key entity.
        * Configuration value entities are immutable and versioned: Changing the value or any metadata leads to a new
          version of the configuration value entity.
        * Before a configuration value entry can be created, the assigned validation logic (part of the the
          configuration key metadata) must be executed. If the validation was successful (return value `true`), the
          entry is persisted; otherwise an error is returned.
* **Caching**
    * A cache entry is the merge result of multiple buckets and specific to one particular cluster.
    * It must be possible to identify in which cache entry a specific configuration value was used.
    * If a configuration value has changed (other attributes coupled to a configuration entry don't matter), any cache
      entries that were using this configuration value must be invalidated.
    * The invalidation of a cache entry leads to a reconciliation for the related clusters (scheduling of a
      reconciliation run).
* **CLI**
    * A CLI is used to access the configuration management.
    * The CLI commands have following syntax: `cmd <verb> <nome> [-flag1] [-flag2=valueOfFlag] ...`.

      Example: `kyco list buckets`,  `kyco add entry -b bucket -k key -v value -t integer` etc.

## Solution Strategy

### Data model

The data of the configuration management can be distinguished in two domains:

* Configuration data

  Any information related to the configuration entires (metadata per configuration key, different versions of
  configuration entries etc.).

* Caching data

  A cache entity contains the merged configuration for one cluster. It also tracks which configuration entry was used in
  which cache entity to identify outdated cluster configurations.

#### Configuration Data

Requirements for the data structure layout:

* Store key and values entities in different versions.
* Map a value entry to a key entry.

**Configuration key table:**

|Column|Description|Data Type|Primary Key|Example|
|---|---|---|---|---|
|key|The configuration key|string|Yes|`my.config.key`|
|version|Incremental version number|Integer|Yes|`1`|
|datatype|Data type of the stored information|String|No|`integer`|
|encrypted|Flag whether the value is plain text or encrypted|Boolean|No|`false`|
|created|Timestamp when the entry was created|Integer|No|`123456789`|
|user|User who created the entry|String|No|`i98765`|
|validator|Optional logic that is executed to validate the value. Return value must be a boolean: `true`=valid / `false`=invalid|String|No|`it >= 1 && it < 10`
|trigger|Optional trigger functions that are executed by the reconciler|String|No|`restartPod("app=appLabel,component=abc")`

**Configuration value table:**

|Column|Description|Data Type|Primary Key|Example|
|---|---|---|---|---|
|key|Foreign key to configuration key|String|Yes|`my.config.key`|
|key_version|Foreign key to configuration key version|Integer|No|`1`|
|version|Incremental version number|Integer|No|`1`|
|bucket|The bucket the configuration entry belongs to|String|Yes|`cust1`|
|value|The value of the configuration entry|String|No|`123`|
|created|Timestamp when the entry was created|Integer|No|`123456789`|
|user|User who created the entry|String|No|`i98765`|

#### Cache Table

Requirements for the data structure layout:

* Cache bucket merge result per cluster.
* Track which configuration value entity is used in which merge result.

**Cache table:**

|Column|Description|Data Type|Primary Key|Example|
|---|---|---|---|---|
|buckets|CSV string of merged buckets|String|No|`default,prod,cust1`|
|cluster|Name of the cluster|String|Yes|`kyma-aws-cust0001`|
|cache|Result of merged buckets|String|No|`my.config.key=123`<br/>`another.config.key=xyz`<br/>...|
|created|Timestamp when the entry was created|Integer|No|`123456789`|

**Cache dependencies table:**

|Column|Description|Data Type|Primary Key|Example|
|---|---|---|---|---|
|bucket|Name of the bucket the configuration value belongs to|String|Yes|`cust1`|
|key|Name of the configuration key|String|Yes|`my.config.key`|
|cluster|Name of the cluster|String|Yes|`kyma-aws-cust0001`|
|created|Timestamp when the entry was created|Integer|No|`123456789`|
