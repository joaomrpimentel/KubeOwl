document.addEventListener('DOMContentLoaded', () => {
    const app = new KubeOwlApp();
    app.init();
});

class KubeOwlApp {
    constructor() {
        this.currentSort = { key: 'name', order: 'asc' };
        this.allPods = [];
    }

    // Inicializa a aplicação.
    init() {
        this.setupTheme();
        this.setupNavigation();
        this.fetchData();
        setInterval(() => this.fetchData(), 5000); // Atualiza os dados a cada 5 segundos.
    }

    // Configura a alternância de tema (claro/escuro).
    setupTheme() {
        const themeToggle = document.getElementById('theme-toggle');
        const sunIcon = '<i class="fas fa-sun"></i>';
        const moonIcon = '<i class="fas fa-moon"></i>';

        const applyTheme = (theme) => {
            if (theme === 'dark') {
                document.documentElement.classList.add('dark');
                themeToggle.innerHTML = sunIcon;
            } else {
                document.documentElement.classList.remove('dark');
                themeToggle.innerHTML = moonIcon;
            }
        };

        const storedTheme = localStorage.getItem('theme');
        const preferredTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        const currentTheme = storedTheme || preferredTheme;
        applyTheme(currentTheme);

        themeToggle.addEventListener('click', () => {
            const isDark = document.documentElement.classList.toggle('dark');
            const newTheme = isDark ? 'dark' : 'light';
            localStorage.setItem('theme', newTheme);
            applyTheme(newTheme);
        });
    }

    // Configura a navegação entre as seções do dashboard.
    setupNavigation() {
        const navLinks = document.querySelectorAll('.nav-link');
        const sections = document.querySelectorAll('.main-section');
        navLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const targetId = link.getAttribute('href').substring(1);

                navLinks.forEach(l => l.classList.remove('active'));
                link.classList.add('active');

                sections.forEach(s => {
                    s.classList.toggle('hidden', s.id !== `${targetId}-section`);
                });
            });
        });
    }

    // Busca os dados da API.
    async fetchData() {
        try {
            const realtimeRes = await fetch('/api/realtime');
            if (!realtimeRes.ok) throw new Error('Falha ao buscar dados da API');
            
            const realtimeData = await realtimeRes.json();
            this.updateRealtimeUI(realtimeData);
        } catch (error) {
            console.error("Erro ao buscar dados:", error);
        }
    }

    // Atualiza a interface com os novos dados recebidos.
    updateRealtimeUI(data) {
        this.allPods = data.pods || [];

        const updateIndicator = document.getElementById('update-indicator');
        updateIndicator.style.backgroundColor = 'var(--green-500)';
        setTimeout(() => { updateIndicator.style.backgroundColor = 'var(--gray-500)'; }, 500);

        document.getElementById('running-status').innerHTML = data.isRunningInCluster 
            ? `<span style="color: var(--green-500);">In-Cluster</span>` 
            : `<span style="color: var(--yellow-500);">Local</span>`;
        document.getElementById('last-updated').innerText = `Atualizado: ${new Date().toLocaleTimeString()}`;
        document.getElementById('nodes-count').innerText = data.nodes?.length || 0;
        document.getElementById('deployments-count').innerText = data.deploymentCount || 0;
        document.getElementById('namespaces-count').innerText = data.namespaceCount || 0;
        
        this.renderCapacityView(data.capacity);
        this.renderNodeList(data.nodes || []);
        this.renderPodTable();
        this.renderIngressesView(data.ingresses || []);
        this.renderEventFeed(data.events || []);
        this.renderStorageView(data.pvcs || []);
    }

    renderCapacityView(capacity) {
        if (!capacity) return;

        const toGiB = (bytes) => bytes / (1024 * 1024 * 1024);
        const toCores = (milli) => milli / 1000;

        document.getElementById('cpu-progress-bar').style.width = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        document.getElementById('cpu-usage-text').innerText = `${toCores(capacity.usedCpu).toFixed(2)} / ${toCores(capacity.totalCpu).toFixed(2)} Cores`;
        document.getElementById('cpu-usage-percentage').innerText = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        
        document.getElementById('memory-progress-bar').style.width = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
        document.getElementById('memory-usage-text').innerText = `${toGiB(capacity.usedMemory).toFixed(2)} / ${toGiB(capacity.totalMemory).toFixed(2)} GiB`;
        document.getElementById('memory-usage-percentage').innerText = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
    }
    
    // Renderiza a lista de nós do cluster com a nova estrutura HTML.
    renderNodeList(nodes) {
        const nodesList = document.getElementById('nodes-list');
        
        if (!nodes || nodes.length === 0) {
            nodesList.innerHTML = '<p>Nenhum nó encontrado.</p>';
            return;
        }

        nodesList.innerHTML = nodes.map(node => `
            <div class="card node-card">
                <div class="node-header">
                    <h4>${node.name}</h4>
                    ${node.role === 'Control-Plane' ? '<span class="node-role">MASTER</span>' : ''}
                </div>
    
                <!-- Métricas de CPU -->
                <div>
                    <div class="node-metric-label">
                        <span>CPU</span>
                        <span style="font-family: monospace;">${node.usedCpu} / ${node.totalCpu} Cores</span>
                    </div>
                    <div class="progress-bar-bg">
                        <div class="progress-bar bg-blue" style="width: ${node.cpuUsagePercentage.toFixed(2)}%"></div>
                    </div>
                </div>
    
                <!-- Métricas de Memória -->
                <div>
                    <div class="node-metric-label">
                        <span>Memória</span>
                        <span style="font-family: monospace;">${node.usedMemory} / ${node.totalMemory}</span>
                    </div>
                    <div class="progress-bar-bg">
                        <div class="progress-bar bg-green" style="width: ${node.memoryUsagePercentage.toFixed(2)}%"></div>
                    </div>
                </div>
                
                <!-- Contagem de Pods -->
                <div class="node-pods-count">
                     <span>Pods em Execução</span>
                     <span class="count">${node.podCount}</span>
                </div>
            </div>
        `).join('');
    }

    getPodStatusClass(status) {
        switch(status) {
            case 'Running':
            case 'Succeeded':
                return 'status-running';
            case 'Pending':
            case 'ContainerCreating':
                return 'status-pending';
            case 'Failed':
            case 'Error':
            case 'CrashLoopBackOff':
                return 'status-failed';
            default:
                return 'status-unknown';
        }
    }
    
    renderPodTable() {
        const tableHeader = document.getElementById('pods-table-header');
        const tableBody = document.getElementById('pods-table-body');
        
        const headers = [
            { name: 'Pod / Namespace', key: 'name' }, { name: 'Nó', key: 'nodeName' }, { name: 'Status', key: 'status' },
            { name: 'Restarts', key: 'restarts' }, { name: 'CPU', key: 'usedCpuMilli' }, { name: 'Memória', key: 'usedMemoryBytes' },
        ];
        
        tableHeader.innerHTML = headers.map(h => 
            `<th data-key="${h.key}">${h.name} ${this.currentSort.key === h.key ? (this.currentSort.order === 'asc' ? '▲' : '▼') : ''}</th>`
        ).join('');
        
        this.allPods.sort((a, b) => {
            let aVal = a[this.currentSort.key]; let bVal = b[this.currentSort.key];
            if (typeof aVal === 'string') return this.currentSort.order === 'asc' ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
            return this.currentSort.order === 'asc' ? aVal - bVal : bVal - aVal;
        });

        tableBody.innerHTML = this.allPods.length ? this.allPods.map(pod => `
            <tr>
                <td><div><b>${pod.name}</b></div><div style="font-size: 0.8rem; color: var(--gray-500);">${pod.namespace}</div></td>
                <td style="font-family: monospace;">${pod.nodeName || 'N/A'}</td>
                <td><span class="status-badge ${this.getPodStatusClass(pod.status)}">${pod.status}</span></td>
                <td style="font-family: monospace; text-align: center;">${pod.restarts}</td>
                <td style="font-family: monospace;">${pod.usedCpu || '-'}</td>
                <td style="font-family: monospace;">${pod.usedMemory || '-'}</td>
            </tr>`
        ).join('') : '<tr><td colspan="6" style="text-align: center; padding: 2rem;">Nenhum pod encontrado.</td></tr>';
        
        document.querySelectorAll('#pods-table-header th').forEach(th => {
            th.addEventListener('click', () => {
                const key = th.dataset.key;
                this.currentSort.order = (this.currentSort.key === key && this.currentSort.order === 'desc') ? 'asc' : 'desc';
                this.currentSort.key = key;
                this.renderPodTable();
            });
        });
    }

    renderIngressesView(ingresses) {
        const ingressesTableBody = document.getElementById('ingresses-table-body');
        ingressesTableBody.innerHTML = ingresses.length ? ingresses.map(ingress => `
             <tr>
                <td>${ingress.namespace}</td>
                <td><b>${ingress.name}</b></td>
                <td style="font-family: monospace;"><a href="http://${ingress.hosts.split(',')[0]}" target="_blank" style="color: var(--blue-500); text-decoration: none;">${ingress.hosts}</a></td>
                <td style="font-family: monospace;">${ingress.service}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" style="text-align: center; padding: 2rem;">Nenhum Ingress encontrado.</td></tr>';
    }

    renderEventFeed(events) {
        const eventsList = document.getElementById('events-list');
        const eventTypeBorders = { 'Normal': 'var(--blue-500)', 'Warning': 'var(--yellow-500)' };
        eventsList.innerHTML = events.length ? events.map(event => `
            <div class="card event-card" style="border-left-color: ${eventTypeBorders[event.type] || 'var(--gray-500)'};">
                <div class="event-header">
                    <b>${event.reason}</b>
                    <span>${event.timestamp}</span>
                </div>
                <p class="event-message"><b>${event.object}:</b> ${event.message}</p>
            </div>`).join('') : '<p>Nenhum evento recente.</p>';
    }

    renderStorageView(pvcs) {
        const pvcsTableBody = document.getElementById('pvcs-table-body');
        pvcsTableBody.innerHTML = pvcs.length ? pvcs.map(pvc => `
             <tr>
                <td>${pvc.namespace}</td>
                <td><b>${pvc.name}</b></td>
                <td><span class="status-badge ${pvc.status === 'Bound' ? 'status-bound' : 'status-pending'}">${pvc.status}</span></td>
                <td style="font-family: monospace;">${pvc.capacity}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" style="text-align: center; padding: 2rem;">Nenhum PVC encontrado.</td></tr>';
    }
}
