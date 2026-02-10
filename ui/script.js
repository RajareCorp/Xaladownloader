/* --------------------------------------------------------------
    Références DOM et Utilitaires
-------------------------------------------------------------- */
const search   = document.getElementById('search');
const results  = document.getElementById('results');
const bar      = document.getElementById('bar');

const pad = (num) => num.toString().padStart(2, '0');

/* --------------------------------------------------------------
    Logique de Recherche
-------------------------------------------------------------- */
search.oninput = async () => {
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
};

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
    if (m.kind === "movie") { 
        download(`/api/download?detail=${m.id}&title=${encodeURIComponent(m.title)}`);
    } else {
        results.scrollIntoView({ behavior: 'smooth' }); 
        results.innerHTML = '<li style="width:100%; text-align:center;">Chargement de la série...</li>';
        try {
            const res = await fetch(`/api/download?detail=${m.id}&infoOnly=true`);
            const sheet = await res.json();
            
            if (sheet.data && sheet.data.items) {
                // On récupère season_count que le backend Go a mappé proprement
                const seasonCount = sheet.data.items.season_count || 1; 
                showSeasonSelector(m, seasonCount); // Plus besoin de urlTemplate ici
            }
        } catch (e) {
            results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Erreur de récupération des détails.</li>';
        }
    }
}

function renderMediaList(items) {
    results.innerHTML = '';
    items.forEach(m => {
        const li = document.createElement('li');
        
        if (m.kind === "tv") {
            li.classList.add('is-series');
            const badge = document.createElement('div');
            badge.className = 'badge-series';
            badge.textContent = 'SÉRIE';
            li.appendChild(badge);
        } else {
            const badge = document.createElement('div');
            badge.className = 'badge-film';
            // m.runtime contient déjà "125 min" grâce au backend Go
            badge.textContent = m.runtime;
            li.appendChild(badge); 
        }

        // Correction m.updated au lieu de m.updatedAt
        const dateStr = m.updated ? new Date(m.updated).toLocaleDateString('fr-FR') : 'Inconnue';
        
        const overlay = document.createElement('div');
        overlay.className = 'media-info-overlay';
        overlay.innerHTML = `Mis à jour le : <br><strong>${dateStr}</strong>`;
        li.appendChild(overlay);

        li.innerHTML += `
            <img src="${m.thumbUrl}" alt="${m.title}" class="thumb">
            <span class="title">${m.title}</span>
        `;

        li.onclick = () => handleMediaClick(m); 
        results.appendChild(li);
    });
}

/* --------------------------------------------------------------
    Sélecteur de Saisons et Épisodes
-------------------------------------------------------------- */
function showSeasonSelector(media, seasonCount) {
    results.innerHTML = ''; 
    const header = document.createElement('h3');
    header.className = 'season-header';
    header.textContent = media.title;
    results.appendChild(header);
    
    // Bouton retour
    const backBtn = document.createElement('li');
    backBtn.className = 'season-item';
    backBtn.style.width = "100%";
    backBtn.innerHTML = '⬅';
    backBtn.onclick = () => search.oninput();
    results.appendChild(backBtn);

    for (let i = 1; i <= seasonCount; i++) {
        const li = document.createElement('li');
        li.className = 'season-item';
        li.innerHTML = `<span class="season-label">Saison ${i}</span>`;
        li.onclick = () => loadEpisodes(media, i);
        results.appendChild(li);
    }
}

async function loadEpisodes(media, seasonNum) {
    results.innerHTML = `<li style="width:100%; text-align:center;">Chargement Saison ${seasonNum}...</li>`;
    
    try {
        const res = await fetch(`/api/episodes?id=${media.id}&num=${seasonNum}`);
        const json = await res.json();
        const episodes = json.data.items.episodes;

        results.innerHTML = `<li style="width:100%; background:var(--accent-primary);" onclick="search.oninput()"><span class="title">⬅ Retour</span></li>`;

        episodes.forEach(ep => {
            const li = document.createElement('li');
            li.style.width = "100%";
            const ePad = pad(ep.episode); // Gardé uniquement pour l'affichage visuel

            li.innerHTML = `<span class="title">Épisode ${ePad} : ${ep.name}</span>`;
            
            li.onclick = () => {
                const finalTitle = `${media.title} S${pad(seasonNum)}E${ePad}`;
                
                // APPEL AU BACKEND : On passe detail (id), season et episode
                const downloadUrl = `/api/download?detail=${media.id}&season=${seasonNum}&episode=${ep.episode}&title=${encodeURIComponent(finalTitle)}`;
                
                download(downloadUrl);
            };
            results.appendChild(li);
        });
    } catch (e) {
        results.innerHTML = '<li style="color:#ff6b6b; width:100%;">Saison non disponible.</li>';
    }
}

function download(apiUrl) {
    bar.hidden = false; bar.value = 0;
    // ... reste de ta fonction download inchangée ...
    const a = document.createElement('a');
    a.href = apiUrl;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
}

window.addEventListener('DOMContentLoaded', loadLastReleases);