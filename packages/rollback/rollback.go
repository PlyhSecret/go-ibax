/*---------------------------------------------------------------------------------------------
 *  Copyright (c) IBAX. All rights reserved.
 *  See LICENSE in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
package rollback

import (
	"bytes"
	"strconv"

	"github.com/IBAX-io/go-ibax/packages/types"

	"github.com/IBAX-io/go-ibax/packages/conf/syspar"
	"github.com/pkg/errors"

	"github.com/IBAX-io/go-ibax/packages/consts"
	"github.com/IBAX-io/go-ibax/packages/converter"
	"github.com/IBAX-io/go-ibax/packages/model"
	log "github.com/sirupsen/logrus"
)

// ToBlockID rollbacks blocks till blockID
func ToBlockID(blockID int64, dbTransaction *model.DbTransaction, logger *log.Entry) error {
	_, err := model.MarkVerifiedAndNotUsedTransactionsUnverified()
	if err != nil {
		logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("marking verified and not used transactions unverified")
		return err
	}

	// roll back our blocks
	for {
		block := &model.Block{}
		blocks, err := block.GetBlocks(blockID, syspar.GetMaxTxCount())
		if err != nil {
			logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting blocks")
			return err
		}
		if len(blocks) == 0 {
			break
		}
		for _, block := range blocks {
			// roll back our blocks to the block blockID
			err = RollbackBlock(block.Data)
			if err != nil {
				return errors.WithMessagef(err, "block_id: %d", block.ID)
			}
			logger.WithFields(log.Fields{"rollback_tx": block.Tx}).Infof("rollback %d successful", block.ID)
		}
		blocks = blocks[:0]
	}
	block := &model.Block{}
	_, err = block.Get(blockID)
	if err != nil {
		logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting block")
		return err
	}

	header, _, err := types.ParseBlockHeader(bytes.NewBuffer(block.Data), syspar.GetMaxBlockSize())
	if err != nil {
		return err
	}

	ib := &model.InfoBlock{
		Hash:           block.Hash,
		BlockID:        header.BlockID,
		Time:           header.Time,
		EcosystemID:    header.EcosystemID,
		KeyID:          header.KeyID,
		NodePosition:   converter.Int64ToStr(header.NodePosition),
		CurrentVersion: strconv.Itoa(header.Version),
		RollbacksHash:  block.RollbacksHash,
	}

	err = ib.Update(dbTransaction)
	if err != nil {
		logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("updating info block")
		return err
	}

	return nil
}
