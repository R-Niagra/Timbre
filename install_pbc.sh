# References:
# https://godoc.org/github.com/Nik-U/pbc
# https://crypto.stanford.edu/pbc/download.html
# https://crypto.stanford.edu/pbc/manual/ch01.html

# Install PBC dependencies
sudo apt install libgmp-dev
sudo apt install build-essential flex bison

# Download and install PBC from source
wget https://crypto.stanford.edu/pbc/files/pbc-0.5.14.tar.gz
tar -xvzf pbc-0.5.14.tar.gz
cd pbc-0.5.14
./configure
make
sudo make install

# Rebuild the search path for libraries
sudo ldconfig
