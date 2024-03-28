WORKSPACE Restaurant (
    DESCRIPTOR RestaurantDescriptor ();
    TABLE Order INHERITS ODoc (
        Year int32,
        Month int32,
        Day int32,
        Waiter ref,
        Items TABLE OrderItems (
            Quantity int32,
            SinglePrice currency,
            Article ref
        )
    );
    VIEW OrderedItems (
        Year int32,
        Month int32,
        Day int32,
        Amount currency,
        PRIMARY KEY ((Year), Month, Day)
    ) AS RESULT OF CalcOrderedItems;
    EXTENSION ENGINE WASM(
        COMMAND NewOrder(Order);
        PROJECTOR CalcOrderedItems AFTER EXECUTE ON NewOrder INTENTS(View(OrderedItems));
    );
)