# Filter Configuration (V2)

The **Filter Configuration (V2)** allows you to define granular rules to include or exclude specific cloud resources (Cloud Items) from your operations. By combining target entity types, cloud providers, and specific properties, you can precisely control which resources are processed.

---

## Configuration Schema (YAML)

Filters are defined in your configuration file under the `filters` key. Each filter rule is evaluated as an **AND** condition across its fields, but contains arrays of values that act as **OR** conditions.

### Fields

| YAML Field | Type | Description |
| --- | --- | --- |
| `filterTypes` | Array of strings | The type of filter action to apply (e.g., include or exclude specific resource types). |
| `cloudTypes` | Array of strings | The cloud providers this rule applies to (e.g., `aws`, `azure`, `gcp`). |
| `filterProperties` | Array of strings | The resource metadata field to evaluate. |
| `filterValues` | Array of strings | Match values for the target properties. |

---

## Allowed Values

### Filter Entity Types (`filterTypes`)

These types define whether the rule is meant to include or exclude a specific class of resource.
`excludeAccess` and `includeAccess` is used for access specific filtering while `excludeInstance`and `includeInstance` is used for everything else.

### Filter Properties (`filterProperties`)

These represent the resource attributes evaluated by the filter values.

| Value | Description | Match Type
| --- | --- | --- |
| `name` | The resource's identifier or name tag. | By default name is treated is a prefix for matching. Alternatively a regexp can be used. A regexp can be specified by naming it `regexp:{{YOUR REGEX GOES HERE}}`|
| `owner` | The owner of the resource (often derived from tags/labels). | Owner is also treated as prefix unless the `-e` flag is specified. | 
| `label` | Cloud provider tags or labels associated with the resource. | Label is also treated as a prefix. |

---

## Example Configuration
The following cleans up resources where the name starts with `ephemeral-something`.

```yaml
---
filters:
  -
    filterTypes: 
      - includeInstance
    cloudTypes:
      - azure
    filterProperties:
      - name
    filterValues:
      - ephemeral-something
```

Here is a Packer specific cleanup example:

```yaml
---
filters:
  -
    filterTypes: 
      - includeInstance
    cloudTypes:
      - aws
    filterProperties:
      - name
    filterValues:
      - "Packer Builder"
  -
    filterTypes: 
      - includeInstance
    cloudTypes:
      - azure
    filterProperties:
      - name
    filterValues:
      - "packer-Resource-Group"
      - "pkr-Resource-Group"
  -
    filterTypes: 
      - includeInstance
    cloudTypes:
      - gcp
    filterProperties:
      - name
    filterValues:
      - "packer-"
```

Regexp example:
```yaml
---
filters:
  - filterTypes: 
      - includeInstance
    cloudTypes:
      - azure
    filterProperties:
      - name
    filterValues:
      - cbimg
  - filterTypes: 
      - includeInstance
    cloudTypes:
      - gcp
    filterProperties:
      - name
    filterValues:
      - regexp:^ephemeral-.*-myresource$
```