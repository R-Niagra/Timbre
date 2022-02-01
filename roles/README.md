# Roles
This package implements the different roles in Timbre, which include not only the official roles from the Timbre whitepaper (miner, poster, storage provder, retriever, bandwidth provider) but also others that are not explicitly stated in the whitepaper. For instance, `syncer` implements the role responsible for synchronizing the blockchain.

Every `role` (which might be implemented as an interface later) should fulfill two contracts. First, upon initialization, it should subscribe to the Timbre node's incoming messages. Second, it should have a `Process()` method that acts as an event handler for incoming messages.
