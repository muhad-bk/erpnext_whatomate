/**
 * Centralized Chart.js setup
 * Import this module in components that need charts to ensure registration happens once
 */
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'

// Register Chart.js components once
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

// Set default options for better tooltip behavior
// This makes tooltips show when hovering near data points, not just exactly on them
ChartJS.defaults.interaction.mode = 'index'
ChartJS.defaults.interaction.intersect = false

// Re-export chart components for convenience
export { Line, Bar, Pie, Doughnut } from 'vue-chartjs'
export { ChartJS }

/** Enterprise / Carbon-like default series colors (blue-forward, neutral grays) */
export const CHART_COLOR_PALETTE = [
  'rgb(15, 98, 254)',
  'rgb(57, 115, 209)',
  'rgb(120, 169, 255)',
  'rgb(82, 82, 82)',
  'rgb(141, 141, 141)',
  'rgb(198, 198, 198)',
  'rgb(218, 30, 40)',
  'rgb(245, 158, 11)',
] as const
