# nabicoin

Launch full service!!!!!!!

source code update : 2019.11.04 23:00 (South of Korea)

blockchain reset   : 2019.11.04 23:00 (South of Korea)

.

Download Nabi-Coin:

.

[linux user download link](http://114.203.210.120/execute_file/nabicoin_linux.tar.gz) 


[windows user download link](http://114.203.210.120/execute_file/nabicoin_windows.zip)

.

.


ETC:

.

for windows user

[git-scm has linux terminal](http://www.git-scm.com) 

.

.

HOW TO USE:

.

create wallet:

./nabicoin createwallet

.

mining:

sudo ./nabicoin startnode -miner (wallet_address) 

.

show amount of coin in wallet_address :

A terminal: mining or sudo ./nabicoin startnode

B terminal:./nabicoin getbalance -address (wallet_address)

.

send coin:

A terminal: mining

B terminal: ./nabicoin send -from (wallet_address 1) -to (wallet_address 2) -amount (coin count)

.

print block infomation in blockchain:

./nabicoin printchain

.

[Based on 'A simplified blockchain implementation in Golang'](https://github.com/Jeiwan/blockchain_go)
