/* --------------------------------------------------------------
    Références DOM et Utilitaires
-------------------------------------------------------------- */
const search   = document.getElementById('search');
const results  = document.getElementById('results');

const pad = (num) => num.toString().padStart(2, '0');

/* --------------------------------------------------------------
    Logique de Recherche
-------------------------------------------------------------- */
search.oninput =  debounce(async () => {
    if (search.value.length < 2) {
        results.innerHTML = '';
        return;
    }
    try {
        const res = await fetch('/api/search?q=' + encodeURIComponent(search.value));
        const items = await res.json();
        renderMediaList(items);
    } catch (e) {
        results.innerHTML = '<li style="color:#ff6b6b; width:100%; text-align:center;">Erreur recherche.</li>';
    }
});

// Utilitaire de debounce
function debounce(func, timeout = 300) {
    let timer;
    return (...args) => {
        clearTimeout(timer);
        timer = setTimeout(() => { func.apply(this, args); }, timeout);
    };
}

async function loadFranchise(id) {
    // Feedback visuel : on peut changer l'état des boutons ici
    results.innerHTML = '<li style="width:100%; text-align:center; padding: 2rem;">' +
                        '<div class="loading-spinner"></div> Chargement du catalogue...</li>';
    
    try {
        const res = await fetch(`/api/franchise?id=${id}`);
        if (!res.ok) throw new Error();
        const items = await res.json();
        
        // On réutilise ton moteur de rendu ultra-propre
        renderMediaList(items);
        
        // On scroll un peu vers les résultats pour le confort mobile
        window.scrollTo({ top: search.offsetTop - 20, behavior: 'smooth' });
        
    } catch (e) {
        results.innerHTML = '<li style="color:var(--accent-primary); width:100%; text-align:center;">' +
                            'Catalogue indisponible pour le moment.</li>';
    }
}

async function loadLastReleases() {
    const container = document.getElementById('last-releases-list');
    try {
        const res = await fetch('/api/last-releases');
        const items = await res.json();
        
        container.innerHTML = '';
        items.forEach(m => {
            const div = document.createElement('div');
            div.className = 'side-item';
            
            // Correction ici : m.updated au lieu de m.updatedAt
            const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : '';

            div.innerHTML = `
                <img src="${m.thumbUrl}">
                <div class="info">
                    <span class="title">${m.title}</span>
                    <span class="date">${dateStr}</span>
                </div>
            `;

            div.onclick = () => handleMediaClick(m); 
            container.appendChild(div);
        });
    } catch (e) {
        container.innerHTML = '<p style="font-size:0.8rem; color:red;">Erreur de chargement</p>';
    }
}

async function handleMediaClick(m) {
    try {
        const res = await fetch(`/api/download?detail=${m.id}&infoOnly=true`);
        const sheet = await res.json();
        const data = sheet.data.items;

        // On utilise le titre de l'API en priorité pour éviter les 'undefined'
        const mediaTitle = data.title || m.title;

        if (m.kind === "movie") {
            showSourceSelector(mediaTitle, data.urls, m.id);
        } else {
            const organizedData = organizeSeries(sheet);
            
            // On met à jour l'objet media avec le vrai titre pour les fonctions suivantes
            const updatedMedia = { ...m, title: mediaTitle };
            showSeasonSelectorFromData(updatedMedia, organizedData);
        }
    } catch (e) {
        console.error(e);
        results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Erreur de récupération des détails.</li>';
    }
}

function organizeSeries(data) {
    const items = data.data.items;
    const organized = {};

    items.urls.forEach(link => {
        // Regex pour capturer S01E02 ou /S1/E2/
        const sMatch = link.url.match(/S(\d+)/i);
        const eMatch = link.url.match(/E(\d+)/i);

        if (sMatch && eMatch) {
            const sNum = parseInt(sMatch[1]);
            const eNum = parseInt(eMatch[1]);

            if (!organized[sNum]) organized[sNum] = {};
            if (!organized[sNum][eNum]) organized[sNum][eNum] = [];

            organized[sNum][eNum].push({
                url: link.url,
                name: link.name
            });
        }
    });

    return {
        title: items.title,
        seasonsCount: items.seasons,
        content: organized
    };
}

/* --------------------------------------------------------------
    Sélecteur de Sources (Qualités / Formats)
-------------------------------------------------------------- */

async function showSourceSelector(title, urls, mediaId, season = "", episode = "") {
    const modal = document.getElementById('source-modal');
    const list = document.getElementById('source-list');
    document.getElementById('modal-title').textContent = title;

    list.innerHTML = '';
    modal.removeAttribute('hidden');

    urls.forEach((source, index) => {
        const li = document.createElement('li');
        li.id = `source-item-${index}`; 
        
        li.style.display = 'flex';
        li.style.justifyContent = 'space-between';
        li.style.alignItems = 'center';
        li.style.padding = '10px';
        li.style.transition = 'all 0.3s ease'; // Pour une transition fluide du grisage
        
        const isM3U8 = source.url.includes('.m3u8');
        const formatBadge = isM3U8 ? '<span class="badge-m3u8">TS</span>' : '<span class="badge-mp4">MP4</span>';
        
        li.innerHTML = `
            <div style="display:flex; align-items:center; gap:10px;">
                <span class="status-dot loading" id="dot-${index}"></span>
                ${formatBadge}
                <span>${source.name}</span>
            </div>
            <div class="actions" style="display:flex; gap:5px;">
                <button class="btn-live" data-url="${source.url}" style="cursor:pointer;">👁️ Live</button>
                <button class="btn-download" style="cursor:pointer;">📥 Download</button>
            </div>
        `;

        // Action bouton LIVE (Nouvel onglet)
        li.querySelector('.btn-live').onclick = (e) => {
            e.stopPropagation();
            window.open(source.url, '_blank');
        };

        // Action bouton DOWNLOAD
        li.querySelector('.btn-download').onclick = (e) => {
            e.stopPropagation();
            if (isM3U8) {
                handleM3U8Download(source.url, title);
            } else {
                // Pour le MP4, on force le téléchargement via l'API ou un attribut
                const downloadUrl = `/api/download?detail=${mediaId}&selectedUrl=${encodeURIComponent(source.url)}&title=${encodeURIComponent(title)}`;
                window.location.href = downloadUrl;
            }
            closeModal();
        };

        list.appendChild(li);
        checkLinkStatus(source.url, index);
    });
}

/**
 * Gère le processus de téléchargement des flux M3U8 (HLS)
 * @param {string} url - L'adresse du flux .m3u8
 * @param {string} title - Le nom du média pour le fichier final
 */
async function handleM3U8Download(url, title) {
    const toast = document.getElementById('m3u8-toast');
    const statusText = document.getElementById('m3u8-status-text');
    
    // 1. Afficher l'interface de suivi
    if (toast) {
        toast.style.display = 'block';
        statusText.textContent = "Initialisation...";
    }

    try {
        // 2. Appeler ton API backend pour démarrer la conversion/téléchargement
        // On ne met pas de "await" ici si l'API est asynchrone et répond immédiatement "OK"
        fetch(`/api/m3u8-download?url=${encodeURIComponent(url)}&title=${encodeURIComponent(title)}`);

        // 3. Créer une boucle de vérification (Polling)
        const checker = setInterval(async () => {
            try {
                const res = await fetch(`/api/m3u8-status?title=${encodeURIComponent(title)}`);
                
                if (!res.ok) throw new Error("Erreur serveur");
                
                const data = await res.json();
                
                // Mise à jour du texte (ex: "15%", "Conversion en cours...", etc.)
                if (statusText) {
                    statusText.textContent = data.status;
                }

                // 4. Si le serveur indique que c'est fini
                if (data.status === "Terminé !" || data.completed === true) {
                    clearInterval(checker);
                    
                    // Optionnel : masquer le toast après un délai
                    setTimeout(() => { 
                        if (toast) toast.style.display = 'none'; 
                    }, 5000);
                }
            } catch (err) {
                console.error("Erreur lors de la vérification du statut:", err);
                if (statusText) statusText.textContent = "Erreur de suivi";
                clearInterval(checker);
            }
        }, 1000); // Vérification toutes les secondes

    } catch (error) {
        console.error("Impossible de lancer le téléchargement M3U8:", error);
        alert("Erreur lors du lancement du téléchargement.");
    }
}

async function checkLinkStatus(url, index) {
    const dot = document.getElementById(`dot-${index}`);
    const row = document.getElementById(`source-item-${index}`);
    
    try {
        const res = await fetch(`/api/check-url?url=${encodeURIComponent(url)}`);
        const data = await res.json();
        
        dot.classList.remove('loading');
        
        if (data.status === "ok") {
            dot.classList.add('online');
        } else {
            setOfflineState(dot, row);
        }
    } catch (e) {
        dot.classList.remove('loading');
        setOfflineState(dot, row);
    }
}

function setOfflineState(dot, row) {
    dot.classList.add('offline');
    if (row) {
        row.style.opacity = "0.4";
        row.style.filter = "grayscale(100%)";
        row.style.pointerEvents = "none"; // Empeche tout clic sur la ligne et ses boutons
        row.style.cursor = "not-allowed";
        
        // Optionnel : Désactiver explicitement les boutons pour le style HTML
        row.querySelectorAll('button').forEach(btn => btn.disabled = true);
    }
}

function closeModal() {
    const modal = document.getElementById('source-modal');
    modal.setAttribute('hidden', 'true');
    // SUPPRESSION de search.oninput() pour garder la liste intacte derrière !
}

function renderMediaList(items) {
    results.innerHTML = items.map(m => {
        const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : 'Inconnue';
        const badge = m.kind === "tv" 
            ? '<div class="badge-series">SÉRIE</div>' 
            : `<div class="badge-film">${m.runtime}</div>`;

        return `
            <li class="${m.kind === 'tv' ? 'is-series' : ''}" data-id="${m.id}">
                ${badge}
                <div class="media-info-overlay">Mis à jour le : <br><strong>${dateStr}</strong></div>
                <img src="${m.thumbUrl}" alt="${m.title}" class="thumb">
                <span class="title">${m.title}</span>
            </li>
        `;
    }).join('');

    // On attache les événements après le rendu global
    Array.from(results.children).forEach((li, index) => {
        li.onclick = () => handleMediaClick(items[index]);
    });
}


/* --------------------------------------------------------------
    Sélecteur de Saisons et Épisodes
-------------------------------------------------------------- */

function showSeasonSelectorFromData(media, organizedData) {
    results.innerHTML = `<h3 class="season-header">${organizedData.title}</h3>`;
    
    // Bouton retour à la recherche
    const backBtn = document.createElement('li');
    backBtn.className = 'season-item';
    backBtn.style.width = "100%";
    backBtn.innerHTML = '⬅ Retour';
    backBtn.onclick = () => search.oninput();
    results.appendChild(backBtn);

    // On affiche les saisons trouvées par le Regex
    const seasons = Object.keys(organizedData.content).sort((a, b) => a - b);
    
    seasons.forEach(sNum => {
        const li = document.createElement('li');
        li.className = 'season-item';
        li.innerHTML = `<span class="season-label">Saison ${sNum}</span>`;
        li.onclick = () => renderEpisodesFromData(media, sNum, organizedData.content[sNum]);
        results.appendChild(li);
    });
}

function renderEpisodesFromData(media, sNum, episodesMap) {
    // Correction : On repasse l'objet 'media' complet au clic sur le retour
    const backLi = document.createElement('li');
    backLi.style.width = "100%";
    backLi.style.background = "var(--accent-primary)";
    backLi.innerHTML = `<span class="title">⬅ Retour aux Saisons</span>`;
    backLi.onclick = () => handleMediaClick(media); 
    
    results.innerHTML = '';
    results.appendChild(backLi);

    const episodes = Object.keys(episodesMap).sort((a, b) => a - b);

    episodes.forEach(eNum => {
        const li = document.createElement('li');
        li.style.width = "100%";
        const sources = episodesMap[eNum];
        
        li.innerHTML = `<span class="title">Épisode ${pad(eNum)}</span>`;
        li.onclick = () => {
            // Ici media.title sera bien défini
            const finalTitle = `${media.title} S${pad(sNum)}E${pad(eNum)}`;
            showSourceSelector(finalTitle, sources, media.id);
        };
        results.appendChild(li);
    });
}

window.addEventListener('DOMContentLoaded', loadLastReleases);