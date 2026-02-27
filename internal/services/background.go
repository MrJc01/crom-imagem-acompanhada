package services

import (
	"log"
	"os"
	"strconv"
	"time"

	"crom-vision/internal/database"
)

func HardDeleteExpired() {
	for {
		time.Sleep(1 * time.Hour)

		// Política de retenção máxima de logs (padrão: 90 dias)
		maxDaysStr := os.Getenv("LOG_RETENTION_DAYS")
		if maxDaysStr == "" {
			maxDaysStr = "90"
		}
		maxDays, _ := strconv.Atoi(maxDaysStr)
		if maxDays <= 0 {
			maxDays = 90
		}

		// 1. Coletar os file_paths primeiro antes de apagar
		rows, err := database.DB.Query(`SELECT file_path FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP AND file_path IS NOT NULL AND file_path != ''`)
		if err == nil {
			for rows.Next() {
				var fPath string
				if err := rows.Scan(&fPath); err == nil && fPath != "" {
					// Reconstruir caminho se necessário
					storagePath := os.Getenv("STORAGE_PATH")
					if storagePath == "" {
						storagePath = "./storage"
					}
					fullPath := fPath
					if len(fPath) > 0 && fPath[0] != '/' && fPath[0] != '.' {
						fullPath = storagePath + "/" + fPath
					}
					os.Remove(fullPath)
				}
			}
			rows.Close()
		}

		// 2. Apagar links expirados e seus logs
		res, err := database.DB.Exec(`DELETE FROM access_logs WHERE link_id IN (SELECT id FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP)`)
		if err == nil {
			rowsLog, _ := res.RowsAffected()
			resLinks, _ := database.DB.Exec(`DELETE FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP`)
			rowsLinks, _ := resLinks.RowsAffected()
			if rowsLog > 0 || rowsLinks > 0 {
				log.Printf("[🧹 AUDITORIA] Hard Delete: %d links expirados, %d logs removidos, arquivos de imagem apagados", rowsLinks, rowsLog)
			}
		} else {
			log.Printf("[CRIT] Falha varrendo banco: %v", err)
		}

		// 3. Política de retenção: apagar logs antigos independente do link
		resOld, err := database.DB.Exec(`DELETE FROM access_logs WHERE accessed_at < datetime('now', '-' || ? || ' days')`, maxDays)
		if err == nil {
			oldRows, _ := resOld.RowsAffected()
			if oldRows > 0 {
				log.Printf("[🧹 RETENÇÃO] %d logs com mais de %d dias removidos", oldRows, maxDays)
			}
		}
	}
}
