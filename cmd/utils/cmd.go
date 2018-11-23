package utils

import (
	"compress/gzip"
	"fmt"
	"github.com/ground-x/go-gxplatform/blockchain"
	"github.com/ground-x/go-gxplatform/blockchain/types"
	"github.com/ground-x/go-gxplatform/log"
	"github.com/ground-x/go-gxplatform/node"
	"github.com/ground-x/go-gxplatform/ser/rlp"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

const (
	importBatchSize = 2500
)

var logger = log.NewModuleLogger(log.CmdUtils)

// Fatalf formats a message to standard error and exits the program.
// The message is also printed to standard output if standard error
// is redirected to a different file.
func Fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		}
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}

func StartNode(stack *node.Node) {
	if err := stack.Start(); err != nil {
		Fatalf("Error starting protocol stack: %v", err)
	}
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		logger.Info("Got interrupt, shutting down...")
		go stack.Stop()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				logger.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
			}
		}
	}()
}

func ImportChain(chain *blockchain.BlockChain, fn string) error {
	// Watch for Ctrl-C while the import is running.
	// If a signal is received, the import will stop at the next batch.
	interrupt := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer close(interrupt)
	go func() {
		if _, ok := <-interrupt; ok {
			logger.Info("Interrupted during import, stopping at next batch")
		}
		close(stop)
	}()
	checkInterrupt := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}

	logger.Info("Importing blockchain", "file", fn)

	// Open the file handle and potentially unwrap the gzip stream
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	var reader io.Reader = fh
	if strings.HasSuffix(fn, ".gz") {
		if reader, err = gzip.NewReader(reader); err != nil {
			return err
		}
	}
	stream := rlp.NewStream(reader, 0)

	// Run actual the import.
	blocks := make(types.Blocks, importBatchSize)
	n := 0
	for batch := 0; ; batch++ {
		// Load a batch of RLP blocks.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		i := 0
		for ; i < importBatchSize; i++ {
			var b types.Block
			if err := stream.Decode(&b); err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("at block %d: %v", n, err)
			}
			// don't import first block
			if b.NumberU64() == 0 {
				i--
				continue
			}
			blocks[i] = &b
			n++
		}
		if i == 0 {
			break
		}
		// Import the batch.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		missing := missingBlocks(chain, blocks[:i])
		if len(missing) == 0 {
			logger.Info("Skipping batch as all blocks present", "batch", batch, "first", blocks[0].Hash(), "last", blocks[i-1].Hash())
			continue
		}
		if _, err := chain.InsertChain(missing); err != nil {
			return fmt.Errorf("invalid block %d: %v", n, err)
		}
	}
	return nil
}

func missingBlocks(chain *blockchain.BlockChain, blocks []*types.Block) []*types.Block {
	head := chain.CurrentBlock()
	for i, block := range blocks {
		// If we're behind the chain head, only check block, state is available at head
		if head.NumberU64() > block.NumberU64() {
			if !chain.HasBlock(block.Hash(), block.NumberU64()) {
				return blocks[i:]
			}
			continue
		}
		// If we're above the chain head, state availability is a must
		if !chain.HasBlockAndState(block.Hash(), block.NumberU64()) {
			return blocks[i:]
		}
	}
	return nil
}

// ExportChain exports a blockchain into the specified file, truncating any data
// already present in the file.
func ExportChain(blockchain *blockchain.BlockChain, fn string) error {
	logger.Info("Exporting blockchain", "file", fn)

	// Open the file handle and potentially wrap with a gzip stream
	fh, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer fh.Close()

	var writer io.Writer = fh
	if strings.HasSuffix(fn, ".gz") {
		writer = gzip.NewWriter(writer)
		defer writer.(*gzip.Writer).Close()
	}
	// Iterate over the blocks and export them
	if err := blockchain.Export(writer); err != nil {
		return err
	}
	logger.Info("Exported blockchain", "file", fn)

	return nil
}

// ExportAppendChain exports a blockchain into the specified file, appending to
// the file if data already exists in it.
func ExportAppendChain(blockchain *blockchain.BlockChain, fn string, first uint64, last uint64) error {
	logger.Info("Exporting blockchain", "file", fn)

	// Open the file handle and potentially wrap with a gzip stream
	fh, err := os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer fh.Close()

	var writer io.Writer = fh
	if strings.HasSuffix(fn, ".gz") {
		writer = gzip.NewWriter(writer)
		defer writer.(*gzip.Writer).Close()
	}
	// Iterate over the blocks and export them
	if err := blockchain.ExportN(writer, first, last); err != nil {
		return err
	}
	logger.Info("Exported blockchain to", "file", fn)
	return nil
}

// TODO-GX Commented out due to mismatched interface.
//// ImportPreimages imports a batch of exported hash preimages into the database.
//func ImportPreimages(db *database.levelDB, fn string) error {
//	logger.Info("Importing preimages", "file", fn)
//
//	// Open the file handle and potentially unwrap the gzip stream
//	fh, err := os.Open(fn)
//	if err != nil {
//		return err
//	}
//	defer fh.Close()
//
//	var reader io.Reader = fh
//	if strings.HasSuffix(fn, ".gz") {
//		if reader, err = gzip.NewReader(reader); err != nil {
//			return err
//		}
//	}
//	stream := rlp.NewStream(reader, 0)
//
//	// Import the preimages in batches to prevent disk trashing
//	preimages := make(map[common.Hash][]byte)
//
//	for {
//		// Read the next entry and ensure it's not junk
//		var blob []byte
//
//		if err := stream.Decode(&blob); err != nil {
//			if err == io.EOF {
//				break
//			}
//			return err
//		}
//		// Accumulate the preimages and flush when enough ws gathered
//		preimages[crypto.Keccak256Hash(blob)] = common.CopyBytes(blob)
//		if len(preimages) > 1024 {
//			rawdb.WritePreimages(db, 0, preimages)
//			preimages = make(map[common.Hash][]byte)
//		}
//	}
//	// Flush the last batch preimage data
//	if len(preimages) > 0 {
//		rawdb.WritePreimages(db, 0, preimages)
//	}
//	return nil
//}
//
//// ExportPreimages exports all known hash preimages into the specified file,
//// truncating any data already present in the file.
//func ExportPreimages(db *database.levelDB, fn string) error {
//	logger.Info("Exporting preimages", "file", fn)
//
//	// Open the file handle and potentially wrap with a gzip stream
//	fh, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
//	if err != nil {
//		return err
//	}
//	defer fh.Close()
//
//	var writer io.Writer = fh
//	if strings.HasSuffix(fn, ".gz") {
//		writer = gzip.NewWriter(writer)
//		defer writer.(*gzip.Writer).Close()
//	}
//	// Iterate over the preimages and export them
//	it := db.NewIteratorWithPrefix([]byte("secure-key-"))
//	for it.Next() {
//		if err := rlp.Encode(writer, it.Value()); err != nil {
//			return err
//		}
//	}
//	logger.Info("Exported preimages", "file", fn)
//	return nil
//}
