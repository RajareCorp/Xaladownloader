package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func CheckForUpdates() {
	resp, err := http.Get(UpdateConfigURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var updateInfo struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&updateInfo)

	if updateInfo.Version > CurrentVersion {
		fmt.Printf("Nouvelle version détectée : %s. Mise à jour en cours...\n", updateInfo.Version)
		err := doUpdate(updateInfo.URL)
		if err != nil {
			log.Printf("Erreur MAJ: %v", err)
		} else {
			fmt.Println("Mise à jour terminée. Relancez l'application.")
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}
	}
}

func doUpdate(url string) error {
	// 1. Obtenir le chemin de l'exécutable actuel
	executablePath, _ := os.Executable()
	oldPath := executablePath + ".old"

	// 2. Télécharger le nouveau binaire
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 3. Renommer l'actuel pour libérer la place
	os.Remove(oldPath) // Supprime une ancienne sauvegarde si elle existe
	err = os.Rename(executablePath, oldPath)
	if err != nil {
		return err
	}

	// 4. Créer le nouveau fichier à l'emplacement d'origine
	newFile, err := os.Create(executablePath)
	if err != nil {
		return err
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, resp.Body)
	if err != nil {
		return err
	}

	// 5. Rendre le fichier exécutable (Linux/Mac)
	os.Chmod(executablePath, 0755)

	return nil
}
