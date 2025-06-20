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

        const currentTheme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
        applyTheme(currentTheme);

        themeToggle.addEventListener('click', () => {
            const newTheme = document.documentElement.classList.toggle('dark') ? 'dark' : 'light';
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
        updateIndicator.classList.add('bg-green-500');
        setTimeout(() => updateIndicator.classList.remove('bg-green-500'), 500);

        document.getElementById('running-status').innerHTML = data.isRunningInCluster ? '<span class="text-green-500">In-Cluster</span>' : '<span class="text-yellow-500">Local</span>';
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

    // Renderiza os medidores de capacidade do cluster (CPU e Memória).
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
    
    // Renderiza a lista de nós do cluster com barras de progresso.
    renderNodeList(nodes) {
        const nodesList = document.getElementById('nodes-list');
        
        if (!nodes || nodes.length === 0) {
            nodesList.innerHTML = '<p class="text-gray-500 col-span-full text-center">Nenhum nó encontrado.</p>';
            return;
        }

        const allCardsHTML = nodes.map(node => `
            <div class="bg-white dark:bg-gray-800 p-5 rounded-xl shadow-lg flex flex-col space-y-4">
                <div class="flex justify-between items-center">
                    <h4 class="font-bold text-blue-500 truncate text-lg">${node.name}</h4>
                    ${node.role === 'Control-Plane' ? '<span class="px-3 py-1 text-xs rounded-full bg-blue-500/20 text-blue-400 font-semibold uppercase tracking-wider">MASTER</span>' : ''}
                </div>
    
                <!-- Métricas de CPU -->
                <div>
                    <div class="flex justify-between items-end mb-1">
                        <span class="text-sm font-semibold text-gray-600 dark:text-gray-300">CPU</span>
                        <span class="text-sm font-mono text-gray-500 dark:text-gray-400">${node.usedCpu} / ${node.totalCpu} Cores</span>
                    </div>
                    <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
                        <div class="bg-blue-600 h-2.5 rounded-full" style="width: ${node.cpuUsagePercentage.toFixed(2)}%"></div>
                    </div>
                </div>
    
                <!-- Métricas de Memória -->
                <div>
                    <div class="flex justify-between items-end mb-1">
                        <span class="text-sm font-semibold text-gray-600 dark:text-gray-300">Memória</span>
                        <span class="text-sm font-mono text-gray-500 dark:text-gray-400">${node.usedMemory} / ${node.totalMemory}</span>
                    </div>
                    <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
                        <div class="bg-green-500 h-2.5 rounded-full" style="width: ${node.memoryUsagePercentage.toFixed(2)}%"></div>
                    </div>
                </div>
                
                <!-- Contagem de Pods -->
                <div class="pt-3 border-t border-gray-200 dark:border-gray-700/50 flex justify-between items-center">
                     <span class="text-sm font-semibold text-gray-600 dark:text-gray-300">Pods em Execução</span>
                     <span class="font-bold text-xl text-gray-800 dark:text-white">${node.podCount}</span>
                </div>
            </div>
        `).join('');
        
        nodesList.innerHTML = allCardsHTML;
    }

    // Define cores para os diferentes status de pods.
    getPodStatusColor(status) {
        switch(status) {
            case 'Running':
            case 'Succeeded':
                return 'bg-green-500/20 text-green-400';
            case 'Pending':
            case 'ContainerCreating':
                return 'bg-yellow-500/20 text-yellow-400';
            case 'Failed':
            case 'Error':
            case 'CrashLoopBackOff':
                return 'bg-red-500/20 text-red-400';
            default:
                return 'bg-gray-500/20 text-gray-400';
        }
    }
    
    // Renderiza a tabela de pods com status detalhado e reinicializações.
    renderPodTable() {
        const tableHeader = document.getElementById('pods-table-header');
        const tableBody = document.getElementById('pods-table-body');
        
        const headers = [
            { name: 'Pod / Namespace', key: 'name' },
            { name: 'Nó', key: 'nodeName' },
            { name: 'Status', key: 'status' },
            { name: 'Restarts', key: 'restarts' },
            { name: 'CPU', key: 'usedCpuMilli' },
            { name: 'Memória', key: 'usedMemoryBytes' },
        ];
        
        tableHeader.innerHTML = headers.map(h => 
            `<th class="py-3 px-4 text-left text-xs font-bold uppercase text-gray-400 dark:text-gray-500 cursor-pointer" data-key="${h.key}">${h.name} ${this.currentSort.key === h.key ? (this.currentSort.order === 'asc' ? '▲' : '▼') : ''}</th>`
        ).join('');
        
        this.allPods.sort((a, b) => {
            let aVal = a[this.currentSort.key];
            let bVal = b[this.currentSort.key];
            if (typeof aVal === 'string') return this.currentSort.order === 'asc' ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
            return this.currentSort.order === 'asc' ? aVal - bVal : bVal - aVal;
        });

        tableBody.innerHTML = this.allPods.length ? this.allPods.map(pod => `
            <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="py-3 px-4"><div class="font-bold text-sm truncate max-w-xs">${pod.name}</div><div class="text-xs text-gray-400">${pod.namespace}</div></td>
                <td class="py-3 px-4 font-mono text-xs">${pod.nodeName || 'N/A'}</td>
                <td class="py-3 px-4"><span class="px-2 py-1 text-xs rounded-full font-semibold ${this.getPodStatusColor(pod.status)}">${pod.status}</span></td>
                <td class="py-3 px-4 font-mono text-center">${pod.restarts}</td>
                <td class="py-3 px-4 font-mono text-sm">${pod.usedCpu || '-'}</td>
                <td class="py-3 px-4 font-mono text-sm">${pod.usedMemory || '-'}</td>
            </tr>`
        ).join('') : '<tr><td colspan="6" class="text-center py-8 text-gray-500">Nenhum pod encontrado.</td></tr>';
        
        document.querySelectorAll('#pods-table-header th').forEach(th => {
            th.addEventListener('click', () => {
                const key = th.dataset.key;
                this.currentSort.order = (this.currentSort.key === key && this.currentSort.order === 'desc') ? 'asc' : 'desc';
                this.currentSort.key = key;
                this.renderPodTable();
            });
        });
    }

    // Renderiza a tabela de Ingresses.
    renderIngressesView(ingresses) {
        const ingressesTableBody = document.getElementById('ingresses-table-body');
        ingressesTableBody.innerHTML = ingresses.length ? ingresses.map(ingress => `
             <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="py-3 px-4">${ingress.namespace}</td>
                <td class="py-3 px-4 font-bold">${ingress.name}</td>
                <td class="py-3 px-4 font-mono"><a href="http://${ingress.hosts.split(',')[0]}" target="_blank" class="text-blue-400 hover:underline">${ingress.hosts}</a></td>
                <td class="py-3 px-4 font-mono">${ingress.service}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" class="text-center py-8 text-gray-500">Nenhum Ingress encontrado.</td></tr>';
    }

    // Renderiza o feed de eventos recentes.
    renderEventFeed(events) {
        const eventsList = document.getElementById('events-list');
        const eventTypeClasses = {
            'Normal': 'border-l-4 border-blue-500',
            'Warning': 'border-l-4 border-yellow-500'
        };
        eventsList.innerHTML = events.length ? events.map(event => `
            <div class="bg-white dark:bg-gray-800 p-4 rounded-lg shadow ${eventTypeClasses[event.type] || 'border-l-4 border-gray-500'}">
                <div class="flex justify-between items-center text-xs text-gray-400 mb-1">
                    <span class="font-bold">${event.reason}</span>
                    <span>${event.timestamp}</span>
                </div>
                <p class="text-sm"><strong class="dark:text-white">${event.object}:</strong> ${event.message}</p>
            </div>`).join('') : '<p class="text-gray-500">Nenhum evento recente.</p>';
    }

    // Renderiza a tabela de PVCs (armazenamento).
    renderStorageView(pvcs) {
        const pvcsTableBody = document.getElementById('pvcs-table-body');
        pvcsTableBody.innerHTML = pvcs.length ? pvcs.map(pvc => `
             <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="py-3 px-4">${pvc.namespace}</td>
                <td class="py-3 px-4 font-bold">${pvc.name}</td>
                <td class="py-3 px-4"><span class="px-2 py-1 text-xs rounded-full ${pvc.status === 'Bound' ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'}">${pvc.status}</span></td>
                <td class="py-3 px-4 font-mono">${pvc.capacity}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" class="text-center py-8 text-gray-500">Nenhum PVC encontrado.</td></tr>';
    }
}
