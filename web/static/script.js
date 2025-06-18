document.addEventListener('DOMContentLoaded', () => {
    const app = new KubeOwlApp();
    app.init();
});

class KubeOwlApp {
    constructor() {
        this.currentSort = { key: 'usedCpuMilli', order: 'desc' };
        this.allPods = [];
    }

    init() {
        this.setupTheme();
        this.setupNavigation();
        this.fetchData();
        setInterval(() => this.fetchData(), 5000);
    }

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
                    if (s.id === `${targetId}-section`) {
                        s.classList.remove('hidden');
                    } else {
                        s.classList.add('hidden');
                    }
                });
            });
        });
    }

    async fetchData() {
        try {
            const realtimeRes = await fetch('/api/realtime');
            if (!realtimeRes.ok) throw new Error('API fetch para tempo real falhou');
            
            const realtimeData = await realtimeRes.json();
            
            this.updateRealtimeUI(realtimeData);
        } catch (error) {
            console.error("Error fetching realtime data:", error);
        }
    }

    updateRealtimeUI(data) {
        this.allPods = data.pods || [];

        const updateIndicator = document.getElementById('update-indicator');
        updateIndicator.classList.add('bg-green-500');
        setTimeout(() => updateIndicator.classList.remove('bg-green-500'), 500);

        document.getElementById('running-status').innerHTML = data.isRunningInCluster ? '<span class="text-green-500">In-Cluster</span>' : '<span class="text-yellow-500">Local</span>';
        document.getElementById('last-updated').innerText = `Atualizado: ${new Date().toLocaleTimeString()}`;
        document.getElementById('nodes-count').innerText = data.nodes?.length || 0;
        document.getElementById('deployments-count').innerText = data.deploymentCount || 0;
        document.getElementById('services-count').innerText = data.serviceCount || 0;
        document.getElementById('namespaces-count').innerText = data.namespaceCount || 0;
        
        this.renderCapacityView(data.capacity);
        this.renderNodeList(data.nodes || []);
        this.renderPodTable();
        this.renderEventFeed(data.events || []);
        this.renderStorageView(data.pvcs || []);
    }

    renderCapacityView(capacity) {
        if (!capacity) return;

        // CPU
        const cpuProgressBar = document.getElementById('cpu-progress-bar');
        const cpuUsageText = document.getElementById('cpu-usage-text');
        const cpuPercentageText = document.getElementById('cpu-usage-percentage');
        
        cpuProgressBar.style.width = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        cpuUsageText.innerText = `${(capacity.usedCpu / 1000).toFixed(2)} / ${(capacity.totalCpu / 1000).toFixed(2)} Cores`;
        cpuPercentageText.innerText = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        
        // Memory
        const memProgressBar = document.getElementById('memory-progress-bar');
        const memUsageText = document.getElementById('memory-usage-text');
        const memPercentageText = document.getElementById('memory-usage-percentage');
        
        const usedMemGiB = capacity.usedMemory / (1024 * 1024 * 1024);
        const totalMemGiB = capacity.totalMemory / (1024 * 1024 * 1024);

        memProgressBar.style.width = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
        memUsageText.innerText = `${usedMemGiB.toFixed(2)} / ${totalMemGiB.toFixed(2)} GiB`;
        memPercentageText.innerText = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
    }
    
    renderNodeList(nodes) {
        const nodesList = document.getElementById('nodes-list');
        nodesList.innerHTML = nodes.length ? nodes.map(node => `
            <div class="content-card bg-white dark:bg-gray-800 p-4 rounded-lg shadow">
                <h4 class="font-bold text-blue-500 truncate">${node.name}</h4>
                <div class="text-sm space-y-2 mt-3 text-gray-500 dark:text-gray-400">
                    <p><strong>CPU:</strong> ${node.usedCpu} / ${node.totalCpu} Cores</p>
                    <p><strong>Memória:</strong> ${node.usedMemory} / ${node.totalMemory}</p>
                    <p><strong>Pods:</strong> ${node.podCount}</p>
                </div>
            </div>`).join('') : '<p class="text-gray-500">Nenhum nó encontrado.</p>';
    }
    
    renderPodTable() {
        const tableHeader = document.getElementById('pods-table-header');
        const tableBody = document.getElementById('pods-table-body');
        
        const headers = [
            { name: 'Pod / Namespace', key: 'name' },
            { name: 'Nó', key: 'nodeName' },
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
                <td class="py-3 px-4 font-mono text-sm">${pod.usedCpu}</td>
                <td class="py-3 px-4 font-mono text-sm">${pod.usedMemory}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" class="text-center py-8 text-gray-500">Nenhum pod encontrado.</td></tr>';
        
        document.querySelectorAll('#pods-table-header th').forEach(th => {
            th.addEventListener('click', () => {
                const key = th.dataset.key;
                if (this.currentSort.key === key) {
                    this.currentSort.order = this.currentSort.order === 'asc' ? 'desc' : 'asc';
                } else {
                    this.currentSort.key = key;
                    this.currentSort.order = 'desc';
                }
                this.renderPodTable();
            });
        });
    }

    renderEventFeed(events) {
        const eventsList = document.getElementById('events-list');
        const eventTypeClasses = {
            'Normal': 'border-l-4 border-blue-500',
            'Warning': 'border-l-4 border-yellow-500'
        }
        eventsList.innerHTML = events.length ? events.map(event => `
            <div class="event-card bg-white dark:bg-gray-800 p-4 rounded-lg shadow ${eventTypeClasses[event.type] || 'border-l-4 border-gray-500'}">
                <div class="flex justify-between items-center text-xs text-gray-400 mb-1">
                    <span class="font-bold">${event.reason}</span>
                    <span>${event.timestamp}</span>
                </div>
                <p class="text-sm"><strong class="dark:text-white">${event.object}:</strong> ${event.message}</p>
            </div>`).join('') : '<p class="text-gray-500">Nenhum evento recente.</p>';
    }

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
