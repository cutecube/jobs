<?php
declare(strict_types=1);
/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

namespace Spiral\Jobs;

use Doctrine\Common\Inflector\Inflector;
use Spiral\Core\Container\SingletonInterface;
use Spiral\Goridge\RPC;
use Spiral\Jobs\Exception\JobException;
use Spiral\RoadRunner\Exception\RoadRunnerException;

class Queue implements QueueInterface, SingletonInterface
{
    const RR_SERVICE = 'jobs';

    /** @var RPC */
    private $rpc;

    /**
     * @param RPC $rpc
     */
    public function __construct(RPC $rpc)
    {
        $this->rpc = $rpc;
    }

    /**
     * @inheritdoc
     */
    public function push(JobInterface $job, Options $options = null): string
    {
        try {
            if (empty($options)) {
                $options = new Options();
            }

            return $this->rpc->call(self::RR_SERVICE . '.Push', [
                'job'     => $this->jobName($job),
                'payload' => $job->serialize(),
                'options' => $options
            ]);
        } catch (RoadRunnerException|\Throwable $e) {
            throw new JobException($e->getMessage(), $e->getCode(), $e);
        }
    }

    /**
     * @param JobInterface $job
     * @return string
     */
    private function jobName(JobInterface $job): string
    {
        $names = explode('\\', get_class($job));
        $names = array_map(function (string $value) {
            return Inflector::camelize($value);
        }, $names);

        return join('.', $names);
    }
}