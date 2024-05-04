Why after running for a while, data pushed to bitcoin layer goes from 1.39kb, to 35.83kb, to 69.87kb. What is the reason for this jump?

1. It jumps from pushing (1, 3) blocks to (1, 79) blocks

the system recognizes roll - ups blocks in store, but why not submitting? because it is stuck in submit attempt loop

that was due to unset private and internal private key

2. Witness field data is empty? If submitted, it should have been something else

```
[0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]
```

because it was fetching from height 0, I need to set another bitcoin start height

3. engine is fetching old btc blocks first from last test run, which disrupt btc height to new test runs
* data pushed to bitcoin needs to define version (use random chain id to distinguish)

4. why node 2 doesn't find block submitted to DA layer?
* node 1 pushes 15 blocks, node 2 is supposed to find it
* fixed with setting same genesis for both nodes

5. DA node GetIDs() is empty in da RetrieveBlocks()
* DA node is getting empty IDs
* DA namespace is the same for both read and write operations. What seems to be the problem here?
* Aggregator submitted all 15 roll ups blocks to DA layer at DA height 1
* This is due to DummyDA implementation of Submit() and GetIds() which stores blobs at exact height, I need to get blobs at exact height

All of this could take
* 433B/block
* 259800B/600 block (assume 1 second roll - ups)
* 259800*64(sats/B) = 16627200 (sats), approx 12k\$/block
* 1 day has 144 btc blocks: 1728000\$/day, 51840000\$/month

I need to optimize data size for proofs