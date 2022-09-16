# btc-block-time

This project collects the height, hash, and timestamp for the Bitcoin blocks.
It uses two API sources for that:

* btc.com
* blockchain.com

The service is using a number of concurrent tasks in a way that the servers do not block the connection.
All the information collected so far is saved locally, so it does not need to retrieve everything every time.
The service gets the latest block mined until now, then check what is the last block it has information about and then asks for the missing ones.

The initial version of the service was using 1 source, then I noticed me getting rate limited, so I added a new one.
But even two sources would not be fast enough to collect the data block by block. 
So I dig in the API to find an endpoint that would answer with more blocks per request, so the request for all the blocks by day were added.

## Block mining time of more than 2 hours

Each Bitcoin block mining is an independent operation, so one taking more than 10 minutes to be mined does not mean the next one will take as well.
Blocks taking more than the usual to mine is an event that does not happen that often, if compared to the total number of blocks.
So it is a good fit for a Poisson distribution, where:
```math
P(X=x) = e^{-\lambda} \frac{\lambda^x}{x!}
```
Considering that usually we have 1 block each 10 minutes, in 2 hours (120 minutes) we would have 12 blocks.
Being 12 the average number of blocks in 120 minutes we have $\lambda=12$, so for no blocks being generated in 120 minutes ( $x=0$ ), then we have:

```math
P(X=0) = 0.00000614421235332821
```
So it has a chance of 1 in 162755 of happening ( $\frac{1}{P(X=0)} = 162754.79141900392$ ),

The service does this math, fetches the blocks than verifies how many times it has happened until now.
Since the genesis block in Bitcoin we had 152 blocks that took more than 2 hours, the longest mining time was 128h39m20s for block 1.
