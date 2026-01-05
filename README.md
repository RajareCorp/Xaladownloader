# 📦 XalaDownloader

Un petit serveur Go qui recherche et télécharge les vidéos de Xalaflix.

**Auteur :** Rajare  
**Version :** v1.0.0 (Fin 2025)

---

## 🔧 Prérequis

| Outil | Version minimale |
|-------|----------------|
| Go | 1.22 (ou plus récent) |
| Git | 2.x (pour cloner le dépôt) |
| Internet | Accès à Internet |

---

## ⬇️ Installation

### 1️⃣ Cloner le dépôt
```bash
git clone https://github.com/rajare/xaladownloader.git
cd xaladownloader
```

# 2️⃣ Télécharger les dépendances Go (goquery)
```bash
go mod tidy   # récupère github.com/PuerkitoBio/goquery
```
# 3️⃣ (Optionnel) Compiler un binaire autonome
```bash
go build -trimpath -ldflags="-s -w" -o xaladownloader.exe .
```
Vous pouvez directement lancer le serveur avec go run main.go si vous ne voulez pas compiler.

## ▶️ Lancement
# Depuis le répertoire du projet
```bash
go run main.go
```
Ou, si vous avez compilé :
```bash
./xaladownloader
```
Le serveur écoute par défaut sur http://localhost:8080.

Vous verrez dans le terminal :

```bash
🚀 XalaDownloader démarre sur :8080
```

## 📜 Licence
Ce projet est publié sous licence MIT. Voir le fichier LICENSE pour les termes complets.

## 📞 Contact
Pseudo : Rajare
GitHub : https://github.com/rajare
N'hésitez pas à ouvrir une issue si vous rencontrez un bug ou avez une suggestion !

Enjoy your downloads! 🚀