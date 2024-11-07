Generic in-memory cache that allows middleware/callbacks.

Originally designed to be used with the [SSE Server](https://github.com/kamludwinski2/simplesse)

## Functionality
- automatic eviction of expired items
    - expiry can be set using **Set**(key, value, expires? _optional_)
- middleware
    - **OnBeforeTick** triggered before each **Maintain** tick
    - **OnAfterTick** triggered after each **Maintain** tick
    - **onCreate** triggered when a new item is added
    - **onUpdate** triggered when an existing item is updated
    - **onDelete** triggered when an existing item is deleted
    - **onExpiry** triggered when an existing item expires
- metrics
    - **hits** number of successful cache calls
    - **misses** number of unsuccessful cache calls (cached item not found)
    - **items** current cache item count
    - **memoryUsageBytes** total memory usage of cached items in bytes

## Usage
Since it uses generics **[not implementing comparable]** _Equals (a, b T) bool_ has to be implemented.

## Example
Can be found **example/main**