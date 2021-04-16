# Configuration Management

## Requirements

* **Buckets**
  * Configuration entries are bundles in buckets (e.g. default-bucket, landscape or customer specific buckets etc.).
  * Configuration entries with the same key can exists in multiple buckets.
  * It has to be possible to merge multiple buckets together (overlay of buckets): configuration entries of a lower layered bucket will be overwritten by an entry with the same key of a higher layered bucket.
  * The number and the ordering of merged buckets can be flexibly defined.
* **Configuration entries**
  * A configuration entry is always a key value pair (e.g. `my.config.key=the value`)
  * A configuration key is always a string.
  * The configuration value can have a specific type (e.g. String, Integer, Boolean).
  * Configuration entries are immutable and represent one version of the entry. Any change of a configuration leads to a new version of the entry.
  * Each configuration entry contains these metadata:
    * Creation date
    * Author
  * Validation logic can be assigned for configuration entry (e.g. checking min-max constraints etc.).
* **Caching**
  * A cache entry is the merge result of multiple buckets.
  * The validation logic which is assigned for configuration entires is always executed before a cache entry got created: if the validation failed, the entry will not be created.
  * For each cache entry is traceable which configuration-entry of which bucket and in which version was used.
  * If a configuration value was changed (other attributes coupled to a configuration entry don't matter), any cache entries which were using this configuration entry have to be invalidated.
  * The invalidation of a cache entry has to inform the reconciler about a need for reconciling a cluster.
* **CLI**
  * A CLI is used to access the configuration management
  * The CLI commands have following syntax: `cmd <verb> <nome> [-flag1] [-flag2=valueOfFlag] ...`.
  
    Example: `kyco get buckets`,  `kyco add entry -b bucket -k key -v value -t integer` etc.

## Solution Strategy

* Data model

The simplest form for addressing the defined requirements require just two data pools:

1. Configuration entries

One table is required to address following points:

* Store configuration entries in versioned way
* Group configuration entries into buckets

Table structure:

|Column|Description|Data Type|Example|
|--|--|--|--|
|id|Primary Key: a combination of bucket, key and version|String|`bucket1-my.key-1`|
|key|The key of the configuration entry|String|`my.config.key`|
|value|The value of the configuration entry|String|`123`|
|bucket|The bucket the configuration entry belongs to|String|`default`|
|datatype|Data type of the stored information|String|`integer`|
|encrypted|Flag whether the value is plain text or encrypted|Boolean|`false`|
|created|Timestamp when the entry was created|Integer|`123456789`|
|user|User who created the entry|String|`i12345`|
|validator|Optional logic which is executed to validated the value. Return value has to be a boolean: `true`=valid / `false`=invalid|String|`value > 100 && value < 1000`
